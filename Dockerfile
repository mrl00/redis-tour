# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

# Copia dependências e baixa módulos (camada cacheada enquanto go.mod não mudar)
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante do código e compila
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o redis-tour .

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19

WORKDIR /app

# Certificados para conexões TLS (Redis Cloud, etc.)
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/redis-tour .

CMD ["./redis-tour"]
