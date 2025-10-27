# Build stage
FROM golang:1.25.3-alpine AS builder

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

# Create the specific binary we need for the final image
RUN CGO_ENABLED=0 GOOS=linux go build -o pvetui ./cmd/pvetui

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata openssh-client

# Build arguments for user ID (defaults to 1000 if not provided)
ARG USER_ID=1000
ARG GROUP_ID=1000

# Create user with matching UID/GID to host user
RUN addgroup -g ${GROUP_ID} -S appgroup && \
    adduser -u ${USER_ID} -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/pvetui .

# Copy any config files if they exist
COPY --from=builder /app/configs ./configs

# Create necessary directories with proper ownership
# Note: logs are now stored in cache directory (XDG-compliant)
RUN mkdir -p /app/cache /app/cache/badger
# RUN mkdir -p /app/cache /app/cache/badger && \
    # chown -R appuser:appgroup /app

# Switch to non-root user
# USER appuser

# Set environment variables
ENV CACHE_DIR=/app/cache
# LOG_DIR removed - logs are now stored in cache directory

# Health check (optional)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep pvetui || exit 1

# Run the application
ENTRYPOINT ["./pvetui"]
