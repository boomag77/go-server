FROM golang:1.24 AS builder

WORKDIR /telegram_server

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o server ./cmd/main.go

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /telegram_server/server /telegram_server/server

RUN mkdir -p /telegram_server/logs

WORKDIR /telegram_server

EXPOSE 8080

CMD ["./server"]
