# WAConnect Go Docker Build
# Multi-stage build for minimal image size

# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o waconnect ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' waconnect
USER waconnect

# Copy binary and static files
COPY --from=builder /app/waconnect .
COPY --from=builder /app/public ./public

# Create sessions directory
RUN mkdir -p ./sessions

# Environment variables
ENV PORT=3200
ENV SESSION_DIR=./sessions
ENV API_KEY=change-me-in-production
ENV DASHBOARD_USER=admin
ENV DASHBOARD_PASS=waconnect123

# Expose port
EXPOSE 3200

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3200/health || exit 1

# Run
CMD ["./waconnect"]
