FROM golang:1.20-slim AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod tidy && go mod verify

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -v -o server .

FROM alpine:3.18

WORKDIR /app
COPY --from=builder /app/server ./server

EXPOSE 8080

CMD ["/app/server"]
