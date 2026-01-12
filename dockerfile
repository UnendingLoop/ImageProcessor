FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/api ./cmd/api
RUN go build -o /bin/worker ./cmd/worker

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/api /usr/local/bin/api
COPY --from=builder /bin/worker /usr/local/bin/worker

COPY .env .env
COPY internal/web /app/internal/web
COPY internal/migrations /app/migrations
EXPOSE 8080
