FROM golang:1.24 AS builder
WORKDIR /app

# Copy the telemetry, middleware, config, health, and types modules first
COPY ./telemetry/ ./telemetry/
COPY ./middleware/ ./middleware/
COPY ./config/ ./config/
COPY ./health/ ./health/
COPY ./types/ ./types/

# Download gin dependency for telemetry module first
WORKDIR /app/telemetry
RUN go mod download github.com/gin-gonic/gin
# Add Grafana Pyroscope client dependency for telemetry
RUN go mod download github.com/grafana/pyroscope-go
RUN go mod tidy

# Copy the context go.mod file
WORKDIR /app
COPY ./context/go.mod ./context/
COPY ./context/main.go ./context/

# Generate go.sum during the build process
WORKDIR /app/context
RUN go mod edit -replace=crochet/telemetry=../telemetry
RUN go mod edit -replace=crochet/middleware=../middleware
RUN go mod edit -replace=crochet/config=../config
RUN go mod edit -replace=crochet/health=../health
RUN go mod edit -replace=crochet/types=../types
RUN go mod tidy

# Download dependencies
RUN go mod download

# Get dependencies
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/trace
RUN go get crochet/telemetry
RUN go get crochet/middleware
RUN go get crochet/config
RUN go get crochet/health
RUN go get crochet/types
RUN go get github.com/gin-gonic/gin
RUN go get github.com/grafana/pyroscope-go

# Install curl
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Build the application with the production build tag
RUN CGO_ENABLED=0 GOOS=linux go build -tags=prod -o main ./main.go

FROM alpine:latest
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/context/main ./main

# Install curl for healthcheck
RUN apk --no-cache add curl

# Expose the port
EXPOSE 8081

# Run the application
CMD ["./main"]
