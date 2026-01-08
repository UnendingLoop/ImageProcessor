# 1. Используем официальный образ Go для сборки
FROM golang:1.24 AS builder

WORKDIR /app

# Копируем go.mod и go.sum отдельно, чтобы кэшировать зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь код и собираем бинарник
COPY . .
RUN go build -o orderservice ./cmd/main.go



# 2. Минимальный образ для запуска
FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/orderservice .

COPY .env .env
COPY internal/web /app/internal/web
COPY internal/kafka/ /app/internal/kafka

EXPOSE 8081

CMD ["./orderservice"]
