FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy module directories
COPY telemetry/ ./telemetry/
COPY middleware/ ./middleware/
COPY config/ ./config/
COPY health/ ./health/
COPY types/ ./types/
COPY httpclient/ ./httpclient/

# Set up telemetry module first
WORKDIR /app/telemetry
RUN go mod download github.com/gin-gonic/gin
RUN go mod download github.com/grafana/pyroscope-go
RUN go mod tidy

# Back to main directory
WORKDIR /app

# Copy workserver module (all files)
COPY workserver/ ./workserver/

# Go to workserver directory to build
WORKDIR /app/workserver

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Fix module paths to use local copies
RUN go mod edit -replace=crochet/telemetry=../telemetry
RUN go mod edit -replace=crochet/middleware=../middleware
RUN go mod edit -replace=crochet/config=../config
RUN go mod edit -replace=crochet/health=../health
RUN go mod edit -replace=crochet/types=../types

# Download dependencies
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/trace
RUN go get crochet/telemetry
RUN go get crochet/middleware
RUN go get crochet/config
RUN go get crochet/types
RUN go get crochet/health
RUN go get github.com/grafana/pyroscope-go
RUN go get github.com/kelseyhightower/envconfig
RUN go get github.com/mitchellh/mapstructure
RUN go mod tidy

# Build the application
RUN go build -tags=prod -o workserver .

# Use a smaller image for the runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/workserver/workserver .

# Expose the port the server listens on
EXPOSE 8084

# Command to run
CMD ["./workserver"]