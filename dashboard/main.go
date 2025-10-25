package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const userCookieName = "spooky-user-id"
const defaultTokens = 10

// IMPORTANT: In a real production environment, this should be loaded from an environment variable or a secure secrets manager, not hardcoded.
const adminSecretKey = "SUPER_SECRET"

// contextKey is a custom type to avoid key collisions in context.
type contextKey string

const userContextKey = contextKey("user")

// User defines the structure for a user in our system.
type User struct {
	ID              string `json:"id"`
	TokensRemaining int    `json:"tokens_remaining"`
	IsAdmin         bool   `json:"is_admin"`
}

// Trigger defines the structure for a single trigger object from the config.
type Trigger struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ArduinoIP   string `json:"arduino_ip"`
	SecretKey   string `json:"secret_key"`
}

// Config defines the top-level structure of the configuration file.
type Config struct {
	Triggers []Trigger `json:"triggers"`
}

// UserStat holds statistics for a single user.
type UserStat struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	IsAdmin   bool      `json:"is_admin"`
	TokensUsed int       `json:"tokens_used"`
}

// ActivationMinute holds activation counts for a single minute.
type ActivationMinute struct {
	Minute      time.Time `json:"minute"`
	PublicCount int       `json:"public_count"`
	AdminCount  int       `json:"admin_count"`
}

// Stats holds the data for the statistics page.
type Stats struct {
	TotalUsers          int                `json:"total_users"`
	TotalRecharges      int                `json:"total_recharges"`
	TriggerActivations  []TriggerStat      `json:"trigger_activations"`
	UserStats           []UserStat         `json:"user_stats"`
	ActivationsLastHour []ActivationMinute `json:"activations_last_hour"`
}

// TriggerStat holds activation counts for a single trigger.
type TriggerStat struct {
	TriggerID   string `json:"trigger_id"`
	TriggerName string `json:"trigger_name"`
	PublicCount int    `json:"public_count"`
	AdminCount  int    `json:"admin_count"`
	FailureCount int   `json:"failure_count"`
}

// App holds application-wide state.
type App struct {
	config     *Config
	db         *sql.DB
	httpClient *http.Client
}

// --- Database and Config Functions ---

func loadConfig() (*Config, error) {
	file, err := os.ReadFile("config/config.json")
	if err != nil {
		return nil, err
	}
	var config Config
	err = json.Unmarshal(file, &config)
	return &config, err
}

func initDB(filepath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	usersTableSQL := `CREATE TABLE IF NOT EXISTS users (
		"id" TEXT NOT NULL PRIMARY KEY,
		"tokens_remaining" INTEGER NOT NULL,
		"is_admin" BOOLEAN NOT NULL DEFAULT 0,
		"created_at" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(usersTableSQL)
	if err != nil {
		return nil, err
	}

	actionsTableSQL := `CREATE TABLE IF NOT EXISTS actions (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"user_id" TEXT NOT NULL,
		"trigger_id" TEXT NOT NULL,
		"timestamp" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		"success" BOOLEAN NOT NULL DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	_, err = db.Exec(actionsTableSQL)
	if err != nil {
		return nil, err
	}

	// --- Schema Migration: Add 'success' column to 'actions' table if it doesn't exist ---
	rows, err := db.Query("PRAGMA table_info(actions)")
	if err != nil {
		return nil, fmt.Errorf("failed to get table info for actions: %w", err)
	}
	defer rows.Close()

	var successColumnExists bool
	for rows.Next() {
		var cid int
		var name string
		var typeName string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typeName, &notnull, &dfltValue, &pk); err == nil && name == "success" {
			successColumnExists = true
			break
		}
	}
	if !successColumnExists {
		log.Println("Schema migration: Adding 'success' column to 'actions' table.")
		_, err = db.Exec(`ALTER TABLE actions ADD COLUMN success BOOLEAN NOT NULL DEFAULT 0`)
		if err != nil {
			return nil, fmt.Errorf("failed to alter actions table: %w", err)
		}
	}

	rechargesTableSQL := `CREATE TABLE IF NOT EXISTS recharges (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"user_id" TEXT NOT NULL,
		"timestamp" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(rechargesTableSQL)
	if err != nil {
		return nil, err
	}

	log.Println("Database tables created or already exist.")
	return db, nil
}

// --- Middleware ---

func (app *App) userAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user *User

		cookie, err := r.Cookie(userCookieName)
		if err != nil {
			newUUID := uuid.New().String()
			user = &User{ID: newUUID, TokensRemaining: defaultTokens, IsAdmin: false}

			_, dbErr := app.db.Exec("INSERT INTO users (id, tokens_remaining, is_admin) VALUES (?, ?, ?)", user.ID, user.TokensRemaining, user.IsAdmin)
			if dbErr != nil {
				log.Printf("ERROR: Failed to create new user: %v", dbErr)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     userCookieName,
				Value:    user.ID,
				Path:     "/",
				Expires:  time.Now().Add(365 * 24 * time.Hour),
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			log.Printf("New user created: ID=%s, Tokens=%d", user.ID, user.TokensRemaining)

		} else {
			userID := cookie.Value
			row := app.db.QueryRow("SELECT id, tokens_remaining, is_admin FROM users WHERE id = ?", userID)
			user = &User{}
			err = row.Scan(&user.ID, &user.TokensRemaining, &user.IsAdmin)
			if err != nil {
				log.Printf("ERROR: User ID from cookie not found in DB: %v", err)
				http.SetCookie(w, &http.Cookie{Name: userCookieName, Path: "/", MaxAge: -1})
				http.Error(w, "Invalid session, please refresh", http.StatusUnauthorized)
				return
			}
			log.Printf("Returning user identified: ID=%s, Tokens=%d", user.ID, user.TokensRemaining)
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- API Handlers ---

func (app *App) triggersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app.config.Triggers)
	}
}

func (app *App) userStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(userContextKey).(*User)
		if !ok {
			http.Error(w, "Could not identify user", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

func (app *App) activateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(userContextKey).(*User)
		if !ok {
			http.Error(w, "Could not identify user", http.StatusInternalServerError)
			return
		}

		if !user.IsAdmin && user.TokensRemaining <= 0 {
			http.Error(w, "You are out of tokens!", http.StatusForbidden)
			return
		}

		triggerID := r.URL.Path[len("/api/activate/"):]
		var targetTrigger *Trigger
		for i := range app.config.Triggers {
			if app.config.Triggers[i].ID == triggerID {
				targetTrigger = &app.config.Triggers[i]
				break
			}
		}
		if targetTrigger == nil {
			http.Error(w, "Trigger not found", http.StatusNotFound)
			return
		}

		// --- Step 1: Spend the token and log the action as pending (success=0) ---
		tx, err := app.db.Begin()
		if err != nil {
			log.Printf("ERROR: activateHandler could not begin transaction: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if !user.IsAdmin {
			res, err := tx.Exec("UPDATE users SET tokens_remaining = tokens_remaining - 1 WHERE id = ?", user.ID)
			if err != nil {
				tx.Rollback()
				log.Printf("ERROR: activateHandler could not decrement tokens: %v", err)
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
			// Verify that a row was actually updated
			rowsAffected, _ := res.RowsAffected()
			if rowsAffected == 0 {
				tx.Rollback()
				log.Printf("ERROR: activateHandler failed to update tokens for user %s (user not found?)", user.ID)
				http.Error(w, "User not found for token update", http.StatusInternalServerError)
				return
			}
		}

		actionRes, err := tx.Exec("INSERT INTO actions (user_id, trigger_id, success) VALUES (?, ?, 0)", user.ID, triggerID)
		if err != nil {
			tx.Rollback()
			log.Printf("ERROR: activateHandler could not insert action: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		actionID, _ := actionRes.LastInsertId()

		if err = tx.Commit(); err != nil {
			log.Printf("ERROR: activateHandler could not commit transaction: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// --- Step 2: Attempt to activate the physical trigger ---
		url := fmt.Sprintf("http://%s/trigger?key=%s", targetTrigger.ArduinoIP, targetTrigger.SecretKey)
		log.Printf("User '%s' activating trigger '%s' (action ID: %d). Tokens before: %d", user.ID, triggerID, actionID, user.TokensRemaining)
		resp, err := app.httpClient.Get(url)

		// --- Step 3: Update status based on success or failure ---
		if err != nil || resp.StatusCode >= 400 {
			// Failure case: Refund the token if the user is not an admin
			if !user.IsAdmin {
				app.db.Exec("UPDATE users SET tokens_remaining = tokens_remaining + 1 WHERE id = ?", user.ID)
				log.Printf("REFUND: Trigger failed for user %s. Token refunded. Tokens now: %d", user.ID, user.TokensRemaining)
			}
			http.Error(w, "Failed to activate physical trigger", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Success case: Mark the action as successful
		app.db.Exec("UPDATE actions SET success = 1 WHERE id = ?", actionID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Trigger '%s' activated successfully!", triggerID)
	}
}

func (app *App) rechargeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(userContextKey).(*User)
		if !ok {
			http.Error(w, "Could not identify user", http.StatusInternalServerError)
			return
		}

		// Use a transaction to ensure both updates happen or neither do.
		tx, err := app.db.Begin()
		if err != nil {
			log.Printf("ERROR: could not begin transaction: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Reset user's tokens
		_, err = tx.Exec("UPDATE users SET tokens_remaining = ? WHERE id = ?", defaultTokens, user.ID)
		if err != nil {
			tx.Rollback()
			log.Printf("ERROR: could not update user tokens on recharge: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Log the recharge action for stats
		_, err = tx.Exec("INSERT INTO recharges (user_id) VALUES (?)", user.ID)
		if err != nil {
			tx.Rollback()
			log.Printf("ERROR: could not insert into recharges table: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if err = tx.Commit(); err != nil {
			log.Printf("ERROR: could not commit recharge transaction: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		log.Printf("User %s recharged tokens.", user.ID)
		user.TokensRemaining = defaultTokens // Update in-memory user struct

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

func (app *App) statsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(userContextKey).(*User)
		if !ok || !user.IsAdmin {
			http.Error(w, "Forbidden: Admins only", http.StatusForbidden)
			return
		}

		var stats Stats
		// Get total users
		app.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
		// Get total recharges
		app.db.QueryRow("SELECT COUNT(*) FROM recharges").Scan(&stats.TotalRecharges)

		// Get all user stats
		userRows, err := app.db.Query(`
			SELECT u.id, u.created_at, u.is_admin, COUNT(a.id)
			FROM users u
			LEFT JOIN actions a ON u.id = a.user_id
			GROUP BY u.id
			ORDER BY u.created_at DESC
		`)
		if err != nil {
			log.Printf("ERROR: could not query user stats: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer userRows.Close()

		for userRows.Next() {
			var us UserStat
			if err := userRows.Scan(&us.ID, &us.CreatedAt, &us.IsAdmin, &us.TokensUsed); err != nil {
				log.Printf("ERROR: scanning user stat row: %v", err)
				continue
			}
			stats.UserStats = append(stats.UserStats, us)
		}

		// Get activations per minute for the last hour
		activationRows, err := app.db.Query(`
			SELECT
				strftime('%Y-%m-%dT%H:%M:00Z', a.timestamp) as minute,
				SUM(CASE WHEN u.is_admin = 0 THEN 1 ELSE 0 END) as public_count,
				SUM(CASE WHEN u.is_admin = 1 THEN 1 ELSE 0 END) as admin_count
			FROM actions a
			JOIN users u ON a.user_id = u.id
			WHERE a.timestamp >= datetime('now', '-1 hour')
			GROUP BY minute
			ORDER BY minute
		`)
		if err != nil {
			log.Printf("ERROR: could not query activation stats: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer activationRows.Close()

		for activationRows.Next() {
			var am ActivationMinute
			var minuteStr string
			if err := activationRows.Scan(&minuteStr, &am.PublicCount, &am.AdminCount); err != nil {
				continue
			}
			am.Minute, _ = time.Parse(time.RFC3339, minuteStr)
			stats.ActivationsLastHour = append(stats.ActivationsLastHour, am)
		}

		// Get trigger activation counts
		rows, err := app.db.Query(`
			SELECT
				a.trigger_id,
				COALESCE(SUM(CASE WHEN u.is_admin = 0 AND a.success = 1 THEN 1 ELSE 0 END), 0) as public_count,
				COALESCE(SUM(CASE WHEN u.is_admin = 1 AND a.success = 1 THEN 1 ELSE 0 END), 0) as admin_count,
				COALESCE(SUM(CASE WHEN a.success = 0 THEN 1 ELSE 0 END), 0) as failure_count
			FROM actions a
			JOIN users u ON a.user_id = u.id
			GROUP BY a.trigger_id
		`)
		if err != nil {
			log.Printf("ERROR: could not query stats: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		triggerNameMap := make(map[string]string)
		for _, t := range app.config.Triggers {
			triggerNameMap[t.ID] = t.Name
		}

		for rows.Next() {
			var ts TriggerStat
			if err := rows.Scan(&ts.TriggerID, &ts.PublicCount, &ts.AdminCount, &ts.FailureCount); err != nil {
				continue // Skip rows with errors
			}
			ts.TriggerName = triggerNameMap[ts.TriggerID]
			stats.TriggerActivations = append(stats.TriggerActivations, ts)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func (app *App) adminLoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			AdminKey string `json:"admin_key"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if payload.AdminKey != adminSecretKey {
			http.Error(w, "Invalid secret key", http.StatusUnauthorized)
			return
		}

		// Key is correct. Create a new admin user and session.
		newUUID := uuid.New().String()
		user := &User{ID: newUUID, TokensRemaining: defaultTokens, IsAdmin: true}

		_, dbErr := app.db.Exec("INSERT INTO users (id, tokens_remaining, is_admin) VALUES (?, ?, ?)", user.ID, user.TokensRemaining, user.IsAdmin)
		if dbErr != nil {
			log.Printf("ERROR: Failed to create new admin user: %v", dbErr)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     userCookieName,
			Value:    user.ID,
			Path:     "/",
			Expires:  time.Now().Add(365 * 24 * time.Hour),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		log.Printf("New admin session created for user %s", user.ID)
		w.WriteHeader(http.StatusOK)
	}
}

func (app *App) adminLogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Expire the user cookie to log them out.
		http.SetCookie(w, &http.Cookie{
			Name:     userCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1, // Tells the browser to delete the cookie immediately.
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		log.Println("User logged out. Cookie expired.")
		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded. Found %d triggers.", len(config.Triggers))

	db, err := initDB("./dashboard.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	app := &App{
		config:     config,
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("./static"))

	// Register API handlers first, so they take precedence over the file server.
	mux.Handle("/api/triggers", app.userAuthMiddleware(app.triggersHandler()))
	mux.Handle("/api/activate/", app.userAuthMiddleware(app.activateHandler()))
	mux.Handle("/api/user/status", app.userAuthMiddleware(app.userStatusHandler()))
	mux.Handle("/api/recharge", app.userAuthMiddleware(app.rechargeHandler()))
	mux.Handle("/api/stats", app.userAuthMiddleware(app.statsHandler()))
	mux.Handle("/api/admin/login", app.adminLoginHandler())
	mux.Handle("/api/admin/logout", app.adminLogoutHandler())
	mux.Handle("/", fs) // The file server should be last to act as a catch-all.

	log.Println("Listening on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
