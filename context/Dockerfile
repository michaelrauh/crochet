FROM golang:1.24

WORKDIR /app

# Copy the context go.mod file
COPY ./context/go.mod ./context/

# Generate go.sum during the build process
WORKDIR /app/context
RUN go mod tidy

# Download dependencies
RUN go mod download

# Copy the source code
COPY ./context/ ./context/

# Build the application
RUN go build -o main ./context/main.go

# Run the application
CMD ["./main"]
