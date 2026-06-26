# Etapa 1: compilar RabbitMQ Writer
FROM golang:1.26.4-bookworm AS builder

WORKDIR /src/rabbit-writer

COPY rabbit-writer/go.mod ./
COPY rabbit-writer/go.sum ./
COPY rabbit-writer/main.go ./

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /out/rabbit-writer .


# Etapa 2: ejecutar el binario
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder \
    /out/rabbit-writer \
    /usr/local/bin/rabbit-writer

EXPOSE 8084

CMD ["rabbit-writer"]