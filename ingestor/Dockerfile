FROM golang:1.24 AS builder

WORKDIR /app

# Copy the text module
COPY ./text ./text

# Copy the source code
COPY ./ingestor/ ./ingestor/

# Set up the build environment
WORKDIR /app/ingestor

# Update the go.mod file with the required dependencies
RUN go mod edit -replace=crochet/text=../text
RUN go get crochet/text
RUN go get github.com/opentracing/opentracing-go
RUN go get github.com/opentracing/opentracing-go/ext
RUN go get github.com/uber/jaeger-client-go
RUN go get github.com/uber/jaeger-client-go/config
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -v -o server ./main.go

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/ingestor/server ./server

EXPOSE 8080

CMD ["/app/server"]
