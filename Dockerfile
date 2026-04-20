# Multi-stage Dockerfile for JennGate
# Stage 1: Builder
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o jenngate ./cmd/jenngate

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates postgresql-client

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/jenngate .

# Copy migrations directory
COPY --from=builder /app/migrations ./migrations

# Create recordings directory
RUN mkdir -p /var/lib/jenngate/recordings && \
    chmod 755 /var/lib/jenngate/recordings

# Expose ports
EXPOSE 8081 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Environment variables
ENV JENNGATE_DB_HOST=postgres
ENV JENNGATE_DB_PORT=5432
ENV JENNGATE_DB_NAME=jenngate
ENV JENNGATE_DB_USER=jenngate
ENV JENNGATE_DB_SSLMODE=require
ENV JENNGATE_HTTP_PORT=8081
ENV JENNGATE_LOG_LEVEL=info
ENV JENNGATE_RECORDING_DIR=/var/lib/jenngate/recordings

# Run the application
CMD ["./jenngate"]
