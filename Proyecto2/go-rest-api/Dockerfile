# Etapa 1: compilar la API REST Go
FROM golang:1.26.4-bookworm AS builder

WORKDIR /src/go-rest-api

COPY go-rest-api/go.mod ./
COPY go-rest-api/main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /out/go-rest-api .


# Etapa 2: ejecutar solamente el binario
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder \
    /out/go-rest-api \
    /usr/local/bin/go-rest-api

EXPOSE 8082

CMD ["go-rest-api"]
