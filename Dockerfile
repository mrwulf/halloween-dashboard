# --- Base Stage ---
# Use an official Go image as a base for both dev and release builds.
FROM golang:1.25-alpine AS base

# Argument to accept the version string
ARG VERSION=dev

WORKDIR /app

# Copy go.mod and go.sum to leverage Docker layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code.
COPY . .

# --- Release Build Stage ---
# This stage builds the final, static binary for production.
FROM base AS release-builder

# Build the static binary for production.
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s -X main.version=${VERSION}" -o /dashboard ./main.go

# --- Development Stage ---
# This stage sets up the live-reloading environment.
FROM base AS dev
RUN apk add --no-cache git && \
    go install github.com/air-verse/air@latest
CMD ["air"]

# --- Final Release Stage ---
# Use a minimal 'distroless' image which contains nothing but our application
# and its runtime dependencies. It runs as a non-root user by default.
FROM gcr.io/distroless/static-debian11

WORKDIR / 
COPY --from=release-builder /dashboard /dashboard
COPY --from=base /app/static /static
# COPY config/config.json.example /config/config.json.example
 
EXPOSE 8080
ENTRYPOINT ["/dashboard"]