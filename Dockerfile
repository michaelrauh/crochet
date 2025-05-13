FROM golang:1.24

WORKDIR /app

COPY . ./

ARG SERVICE
WORKDIR /app/$SERVICE

RUN go mod download
RUN go build -o app

CMD ["./app"]
