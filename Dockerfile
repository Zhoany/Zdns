# -------- Stage 1: build --------
FROM golang:1.24.4 AS builder

WORKDIR /app
COPY . .

ENV CGO_ENABLED=1
RUN go build -o dns-server main.go

# -------- Stage 2: runtime --------
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y libsqlite3-0 ca-certificates && rm -rf /var/lib/apt/lists/* &apt-get update && apt-get install -y tzdata

WORKDIR /app
COPY --from=builder /app/dns-server /app/dns-server

EXPOSE 5300/udp
EXPOSE 5300/tcp
EXPOSE 9898/tcp

CMD ["./dns-server"]
