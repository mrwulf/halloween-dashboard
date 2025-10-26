# --- Build Stage ---
# Use an official Go image to build the application.
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to leverage Docker layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code.
COPY . .

# Build the application as a static binary. This is crucial for running in a minimal image.
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s" -o /dashboard ./main.go

# --- Final Stage ---
# Use a minimal 'scratch' image which contains nothing but our application.
FROM scratch

COPY --from=builder /dashboard /dashboard
COPY static /static
# COPY config/config.json.example /config/config.json.example

EXPOSE 8080
ENTRYPOINT ["/dashboard"]