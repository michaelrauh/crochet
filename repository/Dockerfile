FROM golang:1.24

WORKDIR /app

COPY go.mod ./
COPY . ./

RUN go mod download

RUN go build -o app

CMD ["./app"]
