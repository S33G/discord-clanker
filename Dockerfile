# Multi-stage build for minimal image size
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o bot ./cmd/bot

# Final stage - minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 bot && \
    adduser -D -u 1000 -G bot bot

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=bot:bot /build/bot /app/bot

# Create directories
RUN mkdir -p /app/config /app/data && \
    chown -R bot:bot /app

# Switch to non-root user
USER bot

# Expose no ports (bot connects outbound only)

ENTRYPOINT ["/app/bot"]
CMD ["--config", "/app/config/config.yaml"]
