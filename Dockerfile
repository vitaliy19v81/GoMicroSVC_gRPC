# Используем официальный образ Go для сборки
FROM golang:1.23-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /go_micro_gRPS

# Копируем go.mod и go.sum и скачиваем зависимости
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download

# Копируем все файлы проекта
COPY . .

# Сборка gRPC сервера
RUN go build -o grpc_server ./cmd/api/main.go

# Создаем минимальный образ
FROM alpine:latest

# Устанавливаем рабочую директорию
WORKDIR /root/

# Копируем скомпилированный бинарник
COPY --from=builder /go_micro_gRPS/grpc_server .

# Копируем .env файл
COPY --from=builder /go_micro_gRPS/.env .

# Экспортируем порт
EXPOSE 50051
EXPOSE 8080

# Запуск gRPC сервера
CMD ["./grpc_server"]
