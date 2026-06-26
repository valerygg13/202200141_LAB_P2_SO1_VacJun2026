# Etapa 1: compilar el cliente gRPC
FROM golang:1.26.4-bookworm AS builder

# Esta estructura conserva la relación:
# go-grpc-client/../proto
WORKDIR /src/go-grpc-client

COPY proto /src/proto

COPY go-grpc-client/go.mod ./
COPY go-grpc-client/go.sum ./
COPY go-grpc-client/main.go ./

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /out/go-grpc-client .


# Etapa 2: ejecutar solamente el binario
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder \
    /out/go-grpc-client \
    /usr/local/bin/go-grpc-client

EXPOSE 8083

CMD ["go-grpc-client"]
