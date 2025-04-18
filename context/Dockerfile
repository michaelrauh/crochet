FROM golang:1.19 AS builder

WORKDIR /app

# Copy the context go.mod file
COPY ./context/go.mod ./context/

# Generate go.sum during the build process
WORKDIR /app/context
RUN go mod tidy

# Download dependencies
RUN go mod download

# Install curl
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Copy the source code
COPY ./context/ ./context/

# Install necessary build tools for CGO
RUN apt-get update && apt-get install -y gcc libc6-dev

# Build the application with CGO enabled
RUN CGO_ENABLED=1 go build -o main ./context/main.go

FROM debian:bullseye-slim

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/context/main ./main

# Install runtime dependencies for SQLite and curl
RUN apt-get update && apt-get install -y libsqlite3-0 curl && rm -rf /var/lib/apt/lists/*

# Expose the port
EXPOSE 8081

# Run the application
CMD ["./main"]
