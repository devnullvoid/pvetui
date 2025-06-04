# Build stage
FROM golang:1.24.2-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Debug: Check what was copied
RUN ls -la cmd/proxmox-tui/ || echo "cmd/proxmox-tui directory missing"

# Build using the exact same pattern as CI to verify all packages compile
RUN CGO_ENABLED=0 GOOS=linux go build -v ./...

# Now create the specific binary we need for the final image
RUN CGO_ENABLED=0 GOOS=linux go build -o proxmox-tui ./cmd/proxmox-tui

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Build arguments for user ID (defaults to 1000 if not provided)
ARG USER_ID=1000
ARG GROUP_ID=1000

# Create user with matching UID/GID to host user
RUN addgroup -g ${GROUP_ID} -S appgroup && \
    adduser -u ${USER_ID} -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/proxmox-tui .

# Copy any config files if they exist
COPY --from=builder /app/configs ./configs

# Create necessary directories with proper ownership
RUN mkdir -p /app/cache /app/logs /app/cache/badger
# RUN mkdir -p /app/cache /app/logs /app/cache/badger && \
    # chown -R appuser:appgroup /app

# Switch to non-root user
# USER appuser

# Set environment variables
ENV CACHE_DIR=/app/cache
ENV LOG_DIR=/app/logs

# Health check (optional)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep proxmox-tui || exit 1

# Run the application
ENTRYPOINT ["./proxmox-tui"] 