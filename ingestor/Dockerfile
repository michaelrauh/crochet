FROM golang:1.24 AS builder
WORKDIR /app

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

# Copy the ingestor source code
WORKDIR /app
COPY ./ingestor/go.mod ./ingestor/
COPY ./ingestor/main.go ./ingestor/

# Set up the build environment
WORKDIR /app/ingestor

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
RUN go mod tidy

# Build the application with the production build tag
RUN CGO_ENABLED=0 GOOS=linux go build -tags=prod -v -o server ./main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/ingestor/server .
EXPOSE 8080
CMD ["./server"]
