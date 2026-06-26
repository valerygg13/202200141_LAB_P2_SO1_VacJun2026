# Etapa 1: compilar RabbitMQ Consumer
FROM golang:1.26.4-bookworm AS builder

WORKDIR /src/rabbit-consumer

COPY rabbit-consumer/go.mod ./
COPY rabbit-consumer/go.sum ./
COPY rabbit-consumer/main.go ./

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /out/rabbit-consumer .


# Etapa 2: ejecutar solamente el binario
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder \
    /out/rabbit-consumer \
    /usr/local/bin/rabbit-consumer

CMD ["rabbit-consumer"]
