# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy the telemetry and middleware modules first
COPY telemetry/ ./telemetry/
COPY middleware/ ./middleware/

# Download gin dependency for telemetry module first
WORKDIR /app/telemetry
RUN go mod download github.com/gin-gonic/gin
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

# Get dependencies
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/trace
RUN go get crochet/telemetry
RUN go get crochet/middleware
RUN go mod tidy

# Build the application
RUN go build -o remediations main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/remediations .
EXPOSE 8082
ENV REMEDIATIONS_PORT=8082
CMD ["./remediations"]
