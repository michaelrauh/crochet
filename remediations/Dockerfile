# syntax=docker/dockerfile:1
FROM golang:1.24-alpine as builder
WORKDIR /app
COPY remediations/go.mod .
COPY remediations/main.go .

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Get Jaeger dependencies
RUN go get github.com/opentracing/opentracing-go
RUN go get github.com/opentracing/opentracing-go/ext
RUN go get github.com/uber/jaeger-client-go
RUN go get github.com/uber/jaeger-client-go/config

RUN go build -o remediations main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/remediations .
EXPOSE 8082
ENV REMEDIATIONS_PORT=8082
CMD ["./remediations"]
