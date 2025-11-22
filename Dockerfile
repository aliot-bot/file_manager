FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o file-manager ./cmd/main.go

FROM alpine:3.18

WORKDIR /app

COPY --from=builder /app/file-manager .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/static ./static

VOLUME ["/app/storage"]

EXPOSE 8080

CMD ["./file-manager"]
