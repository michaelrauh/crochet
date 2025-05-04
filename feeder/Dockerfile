FROM golang:1.24.2-alpine AS builder

WORKDIR /app

# Copy only the feeder and its direct dependencies
COPY ./feeder/go.mod ./feeder/
COPY ./config/go.mod ./config/
COPY ./health/go.mod ./health/
COPY ./middleware/go.mod ./middleware/
COPY ./telemetry/go.mod ./telemetry/
COPY ./types/go.mod ./types/
COPY ./httpclient/go.mod ./httpclient/

# Copy the source code
COPY ./feeder/ ./feeder/
COPY ./config/ ./config/
COPY ./health/ ./health/
COPY ./middleware/ ./middleware/
COPY ./telemetry/ ./telemetry/
COPY ./types/ ./types/
COPY ./httpclient/ ./httpclient/

# Build the application
WORKDIR /app/feeder
RUN go mod tidy
RUN go build -o feeder main.go

# Use a smaller image for the final container
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/feeder/feeder .

# Run the application
CMD ["./feeder"]