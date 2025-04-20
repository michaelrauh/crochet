FROM golang:1.24 AS builder
WORKDIR /app

# Copy the telemetry module
COPY ./telemetry ./telemetry

# Copy the text module
COPY ./text ./text

# Copy the httpclient module
COPY ./httpclient ./httpclient

# Copy the config module
COPY ./config ./config

# Copy the source code
COPY ./ingestor/ ./ingestor/

# Set up the build environment
WORKDIR /app/ingestor

# Update the go.mod file with the required dependencies
RUN go mod edit -replace=crochet/text=../text
RUN go mod edit -replace=crochet/telemetry=../telemetry
RUN go mod edit -replace=crochet/httpclient=../httpclient
RUN go mod edit -replace=crochet/config=../config
RUN go get crochet/text
RUN go get crochet/telemetry
RUN go get crochet/httpclient
RUN go get crochet/config

# Add dependencies needed for OpenTelemetry
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/exporters/otlp/otlptrace
RUN go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc

# Add mapstructure dependency
RUN go get github.com/mitchellh/mapstructure

# Add envconfig dependency
RUN go get github.com/kelseyhightower/envconfig
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -v -o server ./main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/ingestor/server ./server
EXPOSE 8080
CMD ["/app/server"]
