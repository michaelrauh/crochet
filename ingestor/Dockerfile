FROM golang:1.24 AS builder

WORKDIR /app

# Copy the ingestor go.mod file
COPY ./ingestor/go.mod ./ingestor/

# Copy the text module
COPY ./text ./text

# Generate go.sum during the build process
WORKDIR /app/ingestor
RUN go mod tidy

# Explicitly add the crochet/text module as a dependency
RUN go get crochet/text

# Download dependencies
RUN go mod download

# Copy the source code
COPY ./ingestor/ ./ingestor/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -v -o server ./ingestor/main.go

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/ingestor/server ./server

EXPOSE 8080

CMD ["/app/server"]
