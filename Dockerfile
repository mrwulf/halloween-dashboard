# --- Base Stage ---
# Use an official Go image as a base for both dev and release builds.
FROM golang:1.25-alpine AS base

# Argument to accept the version string
ARG VERSION=dev

WORKDIR /app

# --- Dependencies Stage ---
# This stage downloads Go modules and is only re-run when go.mod/go.sum change.
FROM base AS deps
COPY go.mod go.sum ./
RUN go mod download

# --- Builder Stage ---
# This stage copies the pre-downloaded modules and the source code.
FROM base AS builder
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY . .

# --- Release Build Stage ---
# This stage builds the final, static binary for production.
FROM builder AS release-builder

# Build the static binary for production.
RUN --mount=type=cache,target=/root/.cache/go-build,from=builder,source=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s -X main.version=${VERSION}" -o /dashboard ./main.go

# --- Development Stage ---
# This stage sets up the live-reloading environment.
FROM builder AS dev
RUN apk add --no-cache git && \
    go install github.com/air-verse/air@latest
CMD ["air"]

# --- Final Release Stage ---
# Use a minimal 'distroless' image which contains nothing but our application
# and its runtime dependencies. It runs as a non-root user by default.
FROM gcr.io/distroless/static-debian11

WORKDIR / 
COPY --from=release-builder /dashboard /dashboard
COPY --from=builder /app/static /static
# COPY config/config.json.example /config/config.json.example
 
EXPOSE 8080
ENTRYPOINT ["/dashboard"]