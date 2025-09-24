# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build both applications
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o youtube-curator ./agents/youtube-curator/cmd
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o drone-weather ./agents/drone-weather/cmd

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and curl for health checks
RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy both binaries from builder stage and set permissions
COPY --from=builder /app/youtube-curator .
COPY --from=builder /app/drone-weather .
RUN chmod +x youtube-curator drone-weather

# Expose health check port (default 8080)
ENV HEALTHCHECK_PORT=8080
EXPOSE 8080

# Health check using HTTP endpoint (configurable via HEALTHCHECK_PORT)
HEALTHCHECK --interval=1m --timeout=30s --start-period=5s --retries=1 \
  CMD curl -f http://localhost:${HEALTHCHECK_PORT}/health || exit 1

# Run the application
CMD ["./youtube-curator"]
