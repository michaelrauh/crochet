# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy the telemetry module first
COPY telemetry/ ./telemetry/

# Copy remediations module files
COPY remediations/go.mod .
COPY remediations/main.go .

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Set up module replacements
RUN go mod edit -replace=crochet/telemetry=./telemetry

# Get OpenTelemetry dependencies
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/trace
RUN go get crochet/telemetry

# Build the application
RUN go build -o remediations main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/remediations .
EXPOSE 8082
ENV REMEDIATIONS_PORT=8082
CMD ["./remediations"]
