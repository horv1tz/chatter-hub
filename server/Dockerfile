# Используем базовый образ Go для сборки
FROM golang:1.23.1-alpine AS builder

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Копируем файлы go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем весь исходный код в рабочую директорию
COPY . .

# Собираем приложение
RUN go build -o server main.go

# Используем минимальный базовый образ для продакшн
FROM alpine:latest

# Устанавливаем необходимые пакеты
RUN apk --no-cache add ca-certificates

# Устанавливаем рабочую директорию
WORKDIR /root/

# Копируем бинарник приложения из предыдущего контейнера
COPY --from=builder /app/server .

# Копируем файлы конфигурации и другие необходимые файлы
COPY .env .env

# Определяем команду для запуска приложения
CMD sh -c "ls && ./server"
