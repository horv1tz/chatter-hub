package config

import (
    "context"
    "log"

    "github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client
var Ctx = context.Background()

// InitRedis инициализирует соединение с Redis
func InitRedis(cfg *Config) {
    RedisClient = redis.NewClient(&redis.Options{
        Addr:     cfg.Redis.Addr,
        Password: cfg.Redis.Password, // Используем пароль для Redis
        DB:       cfg.Redis.DB,
    })

    _, err := RedisClient.Ping(Ctx).Result()
    if err != nil {
        log.Fatalf("Ошибка подключения к Redis: %v", err)
    }
}
