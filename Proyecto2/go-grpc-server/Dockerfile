# Etapa 1: compilar el servidor gRPC
FROM golang:1.26.4-bookworm AS builder

WORKDIR /src/go-grpc-server

# El módulo proto debe quedar como ../proto
COPY proto /src/proto

COPY go-grpc-server/go.mod ./
COPY go-grpc-server/go.sum ./
COPY go-grpc-server/main.go ./

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /out/go-grpc-server .


# Etapa 2: ejecutar el servidor
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder \
    /out/go-grpc-server \
    /usr/local/bin/go-grpc-server

EXPOSE 50051

CMD ["go-grpc-server"]