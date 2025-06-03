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

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o proxmox-tui ./cmd/proxmox-tui

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/proxmox-tui .

# Copy any config files if they exist
COPY --from=builder /app/configs ./configs

# Create necessary directories
RUN mkdir -p /app/cache /app/logs && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose any ports if needed (TUI apps typically don't need ports)
# EXPOSE 8080

# Set environment variables
ENV CACHE_DIR=/app/cache
ENV LOG_DIR=/app/logs

# Health check (optional)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep proxmox-tui || exit 1

# Run the application
ENTRYPOINT ["./proxmox-tui"] 