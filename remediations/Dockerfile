# syntax=docker/dockerfile:1
FROM golang:1.24-alpine as builder
WORKDIR /app
COPY remediations/go.mod .
COPY remediations/main.go .
RUN go build -o remediations main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/remediations .
EXPOSE 8082
ENV REMEDIATIONS_PORT=8082
CMD ["./remediations"]
