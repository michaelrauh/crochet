FROM golang:1.24-alpine as builder

WORKDIR /app

# Install build dependencies for SQLite
RUN apk add --no-cache \
    gcc \
    musl-dev \
    pkgconf \
    sqlite \
    sqlite-dev

# First copy go.mod files to leverage Docker cache for dependencies
COPY go.mod ./
COPY context/go.mod ./context/
COPY types/go.mod ./types/
COPY health/go.mod ./health/
COPY telemetry/go.mod ./telemetry/
COPY httpclient/go.mod ./httpclient/
COPY middleware/go.mod ./middleware/
COPY config/go.mod ./config/
COPY text/go.mod ./text/

# Build without go.work to avoid dependency on other modules
WORKDIR /app/context
RUN go mod edit -replace=crochet/types=../types
RUN go mod edit -replace=crochet/health=../health
RUN go mod edit -replace=crochet/telemetry=../telemetry
RUN go mod edit -replace=crochet/httpclient=../httpclient
RUN go mod edit -replace=crochet/middleware=../middleware
RUN go mod edit -replace=crochet/config=../config
RUN go mod edit -replace=crochet/text=../text

# Run go mod tidy to ensure dependencies are in sync
COPY telemetry/ ./telemetry/
RUN go mod tidy

# Copy source code for our dependencies
WORKDIR /app
COPY context/ ./context/
COPY types/ ./types/
COPY health/ ./health/
COPY telemetry/ ./telemetry/
COPY httpclient/ ./httpclient/
COPY middleware/ ./middleware/
COPY config/ ./config/
COPY text/ ./text/

# Build the application with CGO enabled
WORKDIR /app/context
# Run go mod tidy again after copying all source files to ensure everything is in sync
RUN go mod tidy
RUN CGO_ENABLED=1 GOOS=linux go build -o context_server

# Create a minimal production image
FROM alpine:latest

# Install runtime dependencies for SQLite
RUN apk add --no-cache \
    ca-certificates \
    curl \
    sqlite \
    tzdata

WORKDIR /app

COPY --from=builder /app/context/context_server .

EXPOSE 8081

CMD ["/app/context_server"]
