FROM golang:1.24.2-alpine AS builder

WORKDIR /app

# Install build dependencies for SQLite - matching repository Dockerfile
RUN apk add --no-cache \
    gcc \
    musl-dev \
    pkgconf \
    sqlite \
    sqlite-dev

# Copy only the feeder and its direct dependencies
COPY ./feeder/go.mod ./feeder/
COPY ./config/go.mod ./config/
COPY ./health/go.mod ./health/
COPY ./middleware/go.mod ./middleware/
COPY ./telemetry/go.mod ./telemetry/
COPY ./types/go.mod ./types/
COPY ./httpclient/go.mod ./httpclient/

# Copy the source code
COPY ./feeder/ ./feeder/
COPY ./config/ ./config/
COPY ./health/ ./health/
COPY ./middleware/ ./middleware/
COPY ./telemetry/ ./telemetry/
COPY ./types/ ./types/
COPY ./httpclient/ ./httpclient/

# Build the application
WORKDIR /app/feeder
RUN go mod tidy
# Explicitly add sqlite driver
RUN go get github.com/mattn/go-sqlite3
# Enable CGO for sqlite3 support with production build tag
RUN CGO_ENABLED=1 GOOS=linux go build -tags=prod -v -o feeder

# Use a smaller image for the final container
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies - matching repository Dockerfile
RUN apk add --no-cache \
    ca-certificates \
    curl \
    sqlite

# Copy the binary from the builder stage
COPY --from=builder /app/feeder/feeder .

# Make executable - matching repository Dockerfile
RUN chmod +x /app/feeder

# Run the application
CMD ["./feeder"]