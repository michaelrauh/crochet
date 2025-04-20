FROM golang:1.24 AS builder
WORKDIR /app

# Copy the telemetry and middleware modules first
COPY ./telemetry/ ./telemetry/
COPY ./middleware/ ./middleware/

# Copy the context go.mod file
COPY ./context/go.mod ./context/

# Generate go.sum during the build process
WORKDIR /app/context
RUN go mod edit -replace=crochet/telemetry=../telemetry
RUN go mod edit -replace=crochet/middleware=../middleware
RUN go mod tidy

# Download dependencies
RUN go mod download

# Get dependencies
RUN go get go.opentelemetry.io/otel
RUN go get go.opentelemetry.io/otel/trace
RUN go get crochet/telemetry
RUN go get crochet/middleware
RUN go get github.com/gin-gonic/gin

# Install curl
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Copy the source code
COPY ./context/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./main.go

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
