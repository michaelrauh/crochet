# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy the telemetry, middleware, config, health and types modules first
COPY telemetry/ ./telemetry/
COPY middleware/ ./middleware/
COPY config/ ./config/
COPY health/ ./health/
COPY types/ ./types/

# Download gin dependency for telemetry module first
WORKDIR /app/telemetry
RUN go mod download github.com/gin-gonic/gin
# Add Grafana Pyroscope client dependency for telemetry
RUN go mod download github.com/grafana/pyroscope-go
RUN go mod tidy

# Copy remediations module files
WORKDIR /app
COPY remediations/go.mod .
COPY remediations/main.go .

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Set up module replacements
RUN go mod edit -replace=crochet/telemetry=./telemetry
RUN go mod edit -replace=crochet/middleware=./middleware
RUN go mod edit -replace=crochet/config=./config
RUN go mod edit -replace=crochet/types=./types
RUN go mod edit -replace=crochet/health=./health

# Get dependencies
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/trace
RUN go get crochet/telemetry
RUN go get crochet/middleware
RUN go get crochet/config
RUN go get crochet/types
RUN go get crochet/health

# Add Grafana Pyroscope client dependency
RUN go get github.com/grafana/pyroscope-go
RUN go get github.com/kelseyhightower/envconfig
RUN go get github.com/mitchellh/mapstructure
RUN go mod tidy

# Build the application with the production build tag
RUN go build -tags=prod -o remediations main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/remediations .
# Install curl for the health check
RUN apk add --no-cache curl
EXPOSE 8082
ENV REMEDIATIONS_PORT=8082
CMD ["./remediations"]
