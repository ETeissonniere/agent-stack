# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o youtube-curator ./agents/youtube-curator/cmd

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and curl for health checks
RUN apk --no-cache add ca-certificates curl

# Create a non-root user and app directory
RUN adduser -D -s /bin/sh appuser
RUN mkdir -p /app && chown appuser:appuser /app

WORKDIR /app

# Copy the binary from builder stage and set permissions
COPY --from=builder --chown=appuser:appuser /app/youtube-curator .
RUN chmod +x youtube-curator

# Copy config file
COPY --from=builder --chown=appuser:appuser /app/config.yaml .

USER appuser

# Expose health check port
EXPOSE 8080

# Health check using HTTP endpoint
HEALTHCHECK --interval=1m --timeout=30s --start-period=5s --retries=1 \
  CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./youtube-curator"]