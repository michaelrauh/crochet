FROM golang:1.24 AS builder

WORKDIR /app

# Copy the context go.mod file
COPY ./context/go.mod ./context/

# Generate go.sum during the build process
WORKDIR /app/context
RUN go mod tidy

# Download dependencies
RUN go mod download

# Get Jaeger dependencies
RUN go get github.com/opentracing/opentracing-go
RUN go get github.com/opentracing/opentracing-go/ext
RUN go get github.com/uber/jaeger-client-go
RUN go get github.com/uber/jaeger-client-go/config

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
