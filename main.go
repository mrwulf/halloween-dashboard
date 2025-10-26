package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/fsnotify/fsnotify"
)

// version is set at build time via -ldflags
var version = "dev"

var adminSecretKey = "SUPER_SECRET" // Default value, will be overridden by environment variable.
const userCookieName = "spooky-user-id"
const defaultTokens = 10

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
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Type           string `json:"type"` // e.g., "arduino", "govee_lightning"
	ArduinoIP      string `json:"arduino_ip,omitempty"`
	GoveeDeviceIP  string `json:"govee_device_ip,omitempty"`
	GoveeDeviceID  string `json:"govee_device_id,omitempty"`
	GoveeModel     string `json:"govee_model,omitempty"`
	GoveeColor     *GoveeColorCommandData `json:"govee_color,omitempty"`
	GoveeColorTemp  *int                   `json:"govee_color_temp,omitempty"`
	GoveeBrightness *int                   `json:"govee_brightness,omitempty"`
	GoveeSceneID   *int                   `json:"govee_scene_id,omitempty"`
	SecretKey      string `json:"secret_key"`
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

// GoveeColorCommandData is a helper type for config, distinct from the internal goveeColor type.
type GoveeColorCommandData struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// App holds application-wide state.
type App struct {
	config     *Config
	db         *sql.DB
	httpClient *http.Client

	configMutex sync.RWMutex
}

// --- Govee LAN Control Implementation ---
// This is a self-contained implementation based on community-driven reverse engineering.

const (
	goveePort     = 4003
	goveeListenPort = 4002
	goveeMulticastAddress = "239.255.255.250:4001"
)

type goveeCommand struct {
	Msg struct {
		Cmd  string      `json:"cmd"`
		Data interface{} `json:"data"`
	} `json:"msg"`
}

type goveeColor struct {
	R                int `json:"r"`
	G                int `json:"g"`
	B                int `json:"b"`
	ColorTemperature int `json:"colorTemInKelvin"`
}

type goveeState struct {
	On         int        `json:"onOff"` // Govee sends 0 for off, 1 for on
	Brightness int        `json:"brightness"`
	Color      goveeColor `json:"color"`
}

func sendGoveeCommand(ip string, cmd string, data interface{}) error {
	c := goveeCommand{}
	c.Msg.Cmd = cmd
	c.Msg.Data = data
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal govee command: %w", err)
	}

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", ip, goveePort))
	if err != nil {
		return fmt.Errorf("failed to connect to govee device: %w", err)
	}
	defer conn.Close()

	log.Printf("Sending Govee command to %s: %s", ip, string(jsonBytes))
	_, err = conn.Write(jsonBytes)
	return err
}

func getGoveeStatus(ip string) (*goveeState, error) {
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", goveeListenPort))
	if err != nil {
		return nil, fmt.Errorf("could not resolve govee listen address: %w", err)
	}
	listener, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("could not listen for govee broadcast: %w", err)
	}
	defer listener.Close()

	err = sendGoveeCommand(ip, "devStatus", struct{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to send devStatus command: %w", err)
	}

	buffer := make([]byte, 1024)
	listener.SetReadDeadline(time.Now().Add(3 * time.Second))

	n, _, err := listener.ReadFromUDP(buffer)
	if err != nil {
		return nil, fmt.Errorf("did not receive govee status response: %w", err)
	}

	var resp struct {
		Msg struct {
			Cmd  string     `json:"cmd"`
			Data goveeState `json:"data"`
		} `json:"msg"`
	}

	if err := json.Unmarshal(buffer[:n], &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal govee status response: %w", err)
	}

	if resp.Msg.Cmd != "devStatus" {
		return nil, fmt.Errorf("received unexpected govee command: %s", resp.Msg.Cmd)
	}

	state := resp.Msg.Data
	log.Printf("Govee Status Parsed: Power=%v, Brightness=%d, Color=(%d, %d, %d)", state.On, state.Brightness, state.Color.R, state.Color.G, state.Color.B)
	return &state, nil
}

// --- End Govee Implementation ---

// --- Database and Config Functions ---

func loadConfig() (*Config, error) {
	file, err := os.ReadFile("./config/config.json")
	if err != nil {
		return nil, err
	}
	var config Config
	err = json.Unmarshal(file, &config)
	return &config, err
}

func (app *App) watchConfig() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("ERROR: Failed to create config watcher: %v", err)
		return
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					log.Println("Config file modified. Reloading...")
					app.reloadConfig()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("ERROR: Config watcher error: %v", err)
			}
		}
	}()

	err = watcher.Add("./config/config.json")
	if err != nil {
		log.Printf("ERROR: Failed to add config file to watcher: %v", err)
	}

	// Block forever
	<-make(chan struct{})
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
		app.configMutex.RLock()
		defer app.configMutex.RUnlock()
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
		app.configMutex.RLock()
		var targetTrigger *Trigger
		for i := range app.config.Triggers {
			if app.config.Triggers[i].ID == triggerID {
				targetTrigger = &app.config.Triggers[i]
				break
			}
		}
		app.configMutex.RUnlock()
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

		// --- Step 2: Delegate to the correct trigger type handler ---
		go app.delegateTrigger(targetTrigger, user, actionID)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Trigger '%s' activation initiated!", triggerID)
	}
}

func versionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"version": version,
		})
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

		app.configMutex.RLock()
		defer app.configMutex.RUnlock()
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

		// --- Zero-fill the last hour's data for consistent charting ---
		// Create a map for quick lookups of existing data.
		activationMap := make(map[time.Time]ActivationMinute)
		for _, am := range stats.ActivationsLastHour {
			activationMap[am.Minute] = am
		}

		// Create a complete, zero-filled list for the last 60 minutes.
		var completeActivations []ActivationMinute
		now := time.Now().UTC()
		// Start from 59 minutes ago to create a 60-point dataset that ends *now*.
		for i := 59; i >= 0; i-- { // This loop runs 60 times (59, 58, ..., 0)
			minute := now.Add(time.Duration(-i) * time.Minute).Truncate(time.Minute)
			if data, ok := activationMap[minute]; ok {
				completeActivations = append(completeActivations, data)
			} else {
				completeActivations = append(completeActivations, ActivationMinute{Minute: minute, PublicCount: 0, AdminCount: 0})
			}
		}
		stats.ActivationsLastHour = completeActivations

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

func (app *App) reloadConfig() {
	newConfig, err := loadConfig()
	if err != nil {
		log.Printf("ERROR: Failed to reload config.json: %v. Keeping old configuration.", err)
		return
	}

	app.configMutex.Lock()
	app.config = newConfig
	app.configMutex.Unlock()

	log.Printf("Successfully reloaded configuration. Found %d triggers.", len(newConfig.Triggers))
}

func (app *App) delegateTrigger(trigger *Trigger, user *User, actionID int64) {
	var err error
	triggerType := trigger.Type
	if triggerType == "" {
		triggerType = "arduino" // Default to arduino for backward compatibility
	}

	log.Printf("Delegating action ID %d to handler for type '%s'", actionID, triggerType)

	switch triggerType {
	case "arduino":
		err = app.handleArduinoTrigger(trigger)
	case "govee_lightning":
		err = app.handleGoveeLightningTrigger(trigger)
	case "govee_status":
		err = app.handleGoveeStatusTrigger(trigger)
	case "govee_set_state":
		err = app.handleGoveeSetStateTrigger(trigger)
	default:
		err = fmt.Errorf("unknown trigger type: %s", trigger.Type)
	}

	// --- Step 3: Update status based on success or failure ---
	if err != nil {
		log.Printf("ERROR: Action ID %d failed: %v", actionID, err)
		// Failure case: Refund the token if the user is not an admin
		if !user.IsAdmin {
			app.db.Exec("UPDATE users SET tokens_remaining = tokens_remaining + 1 WHERE id = ?", user.ID)
			log.Printf("REFUND: Trigger failed for user %s. Token refunded. Tokens now: %d", user.ID, user.TokensRemaining)
		}
		// Note: The 'success' column in the 'actions' table remains 0 (the default)
	} else {
		log.Printf("SUCCESS: Action ID %d completed successfully.", actionID)
		// Success case: Mark the action as successful
		app.db.Exec("UPDATE actions SET success = 1 WHERE id = ?", actionID)
	}
}

func (app *App) handleArduinoTrigger(trigger *Trigger) error {
	url := fmt.Sprintf("http://%s/trigger?key=%s", trigger.ArduinoIP, trigger.SecretKey)
	resp, err := app.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to send request to Arduino: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("arduino returned an error status: %s", resp.Status)
	}
	return nil
}

func (app *App) handleGoveeLightningTrigger(trigger *Trigger) (err error) {
	log.Printf("Handling Govee lightning for model '%s'", trigger.GoveeModel)
	return app.simulateGoveeLightning(trigger)
}

func (app *App) simulateGoveeLightning(trigger *Trigger) error {
	log.Printf("Simulating Govee lightning storm on %s", trigger.GoveeDeviceIP)

	initialState, err := getGoveeStatus(trigger.GoveeDeviceIP)
	if err != nil {
		return fmt.Errorf("could not get initial Govee state for simulation: %w", err)
	}
	log.Printf("Govee initial state captured: Power=%d, Brightness=%d", initialState.On, initialState.Brightness)

	// Set a cool white color for the flicker effect.
	if err := sendGoveeCommand(trigger.GoveeDeviceIP, "color", goveeColor{R: 200, G: 200, B: 255}); err != nil {
		log.Printf("Warning: failed to set initial color for flicker: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	effectDuration := 10 * time.Second
	startTime := time.Now()
	for time.Since(startTime) < effectDuration {
		sendGoveeCommand(trigger.GoveeDeviceIP, "brightness", map[string]int{"value": 100})
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

		sendGoveeCommand(trigger.GoveeDeviceIP, "brightness", map[string]int{"value": 1})
		time.Sleep(time.Duration(80+rand.Intn(300)) * time.Millisecond)
	}

	log.Printf("Restoring Govee light to initial state.")
	if err := sendGoveeCommand(trigger.GoveeDeviceIP, "color", initialState.Color); err != nil {
		log.Printf("Warning: failed to restore color: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := sendGoveeCommand(trigger.GoveeDeviceIP, "brightness", map[string]int{"value": initialState.Brightness}); err != nil {
		log.Printf("Warning: failed to restore brightness: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	turnValue := 0
	if initialState.On == 1 {
		turnValue = 1
	}
	return sendGoveeCommand(trigger.GoveeDeviceIP, "turn", map[string]int{"value": turnValue})
}

func (app *App) handleGoveeSetStateTrigger(trigger *Trigger) error {
	log.Printf("Setting Govee state for trigger '%s'", trigger.Name)
	if trigger.GoveeSceneID != nil {
		log.Printf("Activating Govee scene ID: %d", *trigger.GoveeSceneID)
		return sendGoveeCommand(trigger.GoveeDeviceIP, "scene", map[string]int{"value": *trigger.GoveeSceneID})
	}

	// Use a model-specific sequence to set a static color.
	switch trigger.GoveeModel {
	case "H619E":
		// This model requires a specific On -> Brightness -> ColorWC sequence.
		if err := sendGoveeCommand(trigger.GoveeDeviceIP, "turn", map[string]int{"value": 1}); err != nil {
			return fmt.Errorf("failed to turn on H619E: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
		if trigger.GoveeBrightness != nil {
			if err := sendGoveeCommand(trigger.GoveeDeviceIP, "brightness", map[string]int{"value": *trigger.GoveeBrightness}); err != nil {
				return fmt.Errorf("failed to set brightness for H619E: %w", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
		if trigger.GoveeColor != nil {
			colorData := goveeColor{R: trigger.GoveeColor.R, G: trigger.GoveeColor.G, B: trigger.GoveeColor.B}
			if trigger.GoveeColorTemp != nil {
				colorData.ColorTemperature = *trigger.GoveeColorTemp
			}
			return sendGoveeCommand(trigger.GoveeDeviceIP, "colorwc", colorData)
		}
	default: // H6076 and other bulbs
		// This model works best with a Color -> Brightness sequence, which mimics the working lightning storm.
		if trigger.GoveeColor != nil {
			// The H6076 bulb only accepts a simple RGB color command. It does not support color temperature.
			colorData := map[string]int{"r": trigger.GoveeColor.R, "g": trigger.GoveeColor.G, "b": trigger.GoveeColor.B}
			if err := sendGoveeCommand(trigger.GoveeDeviceIP, "color", colorData); err != nil {
				return fmt.Errorf("failed to set color for H6076: %w", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
		if trigger.GoveeBrightness != nil {
			return sendGoveeCommand(trigger.GoveeDeviceIP, "brightness", map[string]int{"value": *trigger.GoveeBrightness})
		}
	}

	return nil
}

func (app *App) handleGoveeStatusTrigger(trigger *Trigger) (err error) {
	_, err = getGoveeStatus(trigger.GoveeDeviceIP)
	return err
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

		// Use the adminSecretKey from env var if available, otherwise the default.
		secret := adminSecretKey
		if envSecret := os.Getenv("ADMIN_SECRET_KEY"); envSecret != "" {
			secret = envSecret
		}
		if payload.AdminKey != secret {
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
		// On logout, create a new public user and issue a new cookie for them.
		// This ensures a clean break from the admin session.
		newUUID := uuid.New().String()
		user := &User{ID: newUUID, TokensRemaining: defaultTokens, IsAdmin: false}

		_, dbErr := app.db.Exec("INSERT INTO users (id, tokens_remaining, is_admin) VALUES (?, ?, ?)", user.ID, user.TokensRemaining, user.IsAdmin)
		if dbErr != nil {
			log.Printf("ERROR: Failed to create new public user on logout: %v", dbErr)
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

		log.Printf("Admin logged out. New public session created for user %s", user.ID)
		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	log.Printf("Starting Haunted Maze Control Dashboard version: %s", version)

	// Seed the random number generator for more natural random effects.
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Override default admin secret if environment variable is set.
	if envSecret := os.Getenv("ADMIN_SECRET_KEY"); envSecret != "" {
		adminSecretKey = envSecret
		log.Println("Loaded ADMIN_SECRET_KEY from environment.")
	}

	// Ensure the data directory exists for the database.
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded. Found %d triggers.", len(config.Triggers))

	db, err := initDB("./data/dashboard.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	app := &App{
		config:     config,
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	go app.watchConfig()

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
	mux.Handle("/api/version", versionHandler())
	mux.Handle("/", fs) // The file server should be last to act as a catch-all.

	log.Println("Listening on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
