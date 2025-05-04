FROM golang:1.24.0 AS builder
WORKDIR /app

# Install SQLite and build dependencies
RUN apt-get update && apt-get install -y sqlite3 libsqlite3-dev

# Copy all shared modules
COPY ./telemetry ./telemetry
COPY ./text ./text
COPY ./httpclient ./httpclient
COPY ./config ./config
COPY ./middleware ./middleware
COPY ./types ./types
COPY ./clients ./clients
COPY ./health ./health

# Download gin dependency for telemetry module first
WORKDIR /app/telemetry
RUN go mod download github.com/gin-gonic/gin

# Add Grafana Pyroscope client dependency for telemetry
RUN go mod download github.com/grafana/pyroscope-go
RUN go mod tidy

# Copy the repository source code
WORKDIR /app
COPY ./repository/go.mod ./repository/
# Copy all Go files from the repository directory instead of just main.go
COPY ./repository/*.go ./repository/

# Set up the build environment
WORKDIR /app/repository

# Update the go.mod file with the required dependencies
RUN go mod edit -replace=crochet/text=../text
RUN go mod edit -replace=crochet/telemetry=../telemetry
RUN go mod edit -replace=crochet/httpclient=../httpclient
RUN go mod edit -replace=crochet/config=../config
RUN go mod edit -replace=crochet/middleware=../middleware
RUN go mod edit -replace=crochet/types=../types
RUN go mod edit -replace=crochet/clients=../clients
RUN go mod edit -replace=crochet/health=../health

# Get dependencies
RUN go get crochet/text
RUN go get crochet/telemetry
RUN go get crochet/httpclient
RUN go get crochet/config
RUN go get crochet/middleware
RUN go get crochet/types
RUN go get crochet/clients
RUN go get crochet/health

# Add dependencies needed for OpenTelemetry
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/exporters/otlp/otlptrace
RUN go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc

# Add Grafana Pyroscope client dependency
RUN go get github.com/grafana/pyroscope-go

# Add mapstructure dependency
RUN go get github.com/mitchellh/mapstructure

# Add envconfig dependency
RUN go get github.com/kelseyhightower/envconfig

# Add Ristretto cache dependency for OrthosCache
RUN go get github.com/dgraph-io/ristretto

# Add SQLite dependency
RUN go get github.com/mattn/go-sqlite3
RUN go mod tidy

# Build the application with the production build tag and CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -tags=prod -v -o server

# Use a more complete Alpine image to include necessary libraries for SQLite
FROM alpine:3.19

# Install runtime dependencies for SQLite and other libraries
RUN apk --no-cache add \
    ca-certificates \
    sqlite \
    sqlite-libs \
    libc6-compat \
    gcompat

WORKDIR /root/
COPY --from=builder /app/repository/server .
EXPOSE 8080
CMD ["./server"]