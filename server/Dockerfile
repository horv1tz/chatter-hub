# Этап сборки
FROM golang:1.23-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum отдельно для использования Docker-кэша
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod tidy

# Копируем остальной исходный код
COPY . .

# Собираем приложение
RUN go build -o app .

# Этап выполнения
FROM alpine:latest

# Устанавливаем необходимые зависимости
RUN apk --no-cache add ca-certificates

# Создаём рабочую директорию
WORKDIR /root/

# Копируем собранное приложение из предыдущего этапа
COPY --from=builder /app/app .

# Открываем порт
EXPOSE 8080

# Команда для запуска приложения
CMD ["./app"]
