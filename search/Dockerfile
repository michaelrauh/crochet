FROM golang:1.24 AS builder
WORKDIR /app

# Copy the telemetry, middleware, config, health, types, and clients modules first
COPY ./telemetry/ ./telemetry/
COPY ./middleware/ ./middleware/
COPY ./config/ ./config/
COPY ./health/ ./health/
COPY ./types/ ./types/
COPY ./clients/ ./clients/
COPY ./httpclient/ ./httpclient/

# Download gin dependency for telemetry module first
WORKDIR /app/telemetry
RUN go mod download github.com/gin-gonic/gin
# Add Grafana Pyroscope client dependency for telemetry
RUN go mod download github.com/grafana/pyroscope-go
RUN go mod tidy

# Copy all files from search directory
WORKDIR /app
COPY ./search/ ./search/

# Generate go.sum during the build process
WORKDIR /app/search
RUN go mod edit -replace=crochet/telemetry=../telemetry
RUN go mod edit -replace=crochet/middleware=../middleware
RUN go mod edit -replace=crochet/config=../config
RUN go mod edit -replace=crochet/health=../health
RUN go mod edit -replace=crochet/types=../types
RUN go mod edit -replace=crochet/clients=../clients
RUN go mod edit -replace=crochet/httpclient=../httpclient
RUN go mod tidy

# Build the application with the production build tag
RUN CGO_ENABLED=0 GOOS=linux go build -v -o main .

FROM alpine:latest
WORKDIR /root/

# Install curl for healthcheck
RUN apk add --no-cache curl

# Copy the built binary from the builder stage
COPY --from=builder /app/search/main .

# Expose the port (match docker-compose.yml)
EXPOSE 8085

# Run the application
CMD ["./main"]