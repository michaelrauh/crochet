FROM golang:1.24-alpine as builder
WORKDIR /app
# Copy shared libraries and dependencies first
COPY telemetry/ ./telemetry/
COPY middleware/ ./middleware/
COPY config/ ./config/
COPY health/ ./health/
COPY types/ ./types/
COPY clients/ ./clients/
COPY httpclient/ ./httpclient/
# Set up telemetry dependencies first (similar to other services)
WORKDIR /app/telemetry
RUN go mod download github.com/gin-gonic/gin
RUN go mod download github.com/grafana/pyroscope-go
RUN go mod tidy
# Back to app directory
WORKDIR /app
# Copy orthos specific files
COPY orthos/go.mod .
COPY orthos/main.go .
# Install necessary build tools
RUN apk add --no-cache gcc musl-dev
# Set up module replacements to use local copies
RUN go mod edit -replace=crochet/telemetry=./telemetry
RUN go mod edit -replace=crochet/middleware=./middleware
RUN go mod edit -replace=crochet/config=./config
RUN go mod edit -replace=crochet/types=./types
RUN go mod edit -replace=crochet/health=./health
RUN go mod edit -replace=crochet/clients=./clients
RUN go mod edit -replace=crochet/httpclient=./httpclient
# Get dependencies
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/trace
RUN go get crochet/telemetry
RUN go get crochet/middleware
RUN go get crochet/config
RUN go get crochet/types
RUN go get crochet/health
RUN go get crochet/clients
RUN go get crochet/httpclient
RUN go get github.com/grafana/pyroscope-go
RUN go get github.com/kelseyhightower/envconfig
RUN go get github.com/mitchellh/mapstructure
# Tidy up modules and build
RUN go mod tidy
RUN go build -tags=prod -o orthos main.go
# Use a small image for the final container
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
# Copy the binary from the builder stage
COPY --from=builder /app/orthos .
# Expose the port
EXPOSE 8083
# Run the binary
CMD ["./orthos"]