package config

import (
    "log"
    "os"
    "strconv"
    "github.com/joho/godotenv"
)

type Config struct {
    Minio    MinioConfig
    Postgres PostgresConfig
    Redis    RedisConfig
    API      APIConfig
    JWT      JWTConfig
}

type MinioConfig struct {
    Endpoint  string
    AccessKey string
    SecretKey string
    UseSSL    bool
}

type PostgresConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    DBName   string
}

type RedisConfig struct {
    Addr     string
    Password string
    DB       int
}

type APIConfig struct {
    Host string
    Port string
}

type JWTConfig struct {
    SecretKey string
    ExpiresIn int64 // в секундах
}

// LoadConfig загружает конфигурацию из .env
func LoadConfig() (*Config, error) {
    err := godotenv.Load()
    if err != nil {
        log.Println("Ошибка загрузки файла .env, будет использоваться системное окружение")
    }

    cfg := &Config{
        Minio: MinioConfig{
            Endpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
            AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
            SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
            UseSSL:    getEnvBool("MINIO_USE_SSL", false),
        },
        Postgres: PostgresConfig{
            Host:     getEnv("POSTGRES_HOST", "localhost"),
            Port:     getEnv("POSTGRES_PORT", "5432"),
            User:     getEnv("POSTGRES_USER", "postgres"),
            Password: getEnv("POSTGRES_PASSWORD", "postgres"),
            DBName:   getEnv("POSTGRES_DB", "postgres"),
        },
        Redis: RedisConfig{
            Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
            Password: getEnv("REDIS_PASSWORD", ""),
            DB:       getEnvInt("REDIS_DB", 0),
        },
        API: APIConfig{
            Host: getEnv("API_HOST", "localhost"),
            Port: getEnv("API_PORT", "8080"),
        },
        JWT: JWTConfig{
			SecretKey: getEnv("JWT_SECRET_KEY", "your-secret-key"),
			ExpiresIn: getEnvInt64("JWT_EXPIRES_IN", 1800), // 1800 секунд = 30 минут
		},
    }

    return cfg, nil
}

// Вспомогательные функции для получения переменных окружения с дефолтными значениями
func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    if value, exists := os.LookupEnv(key); exists {
        intValue, err := strconv.Atoi(value)
        if err == nil {
            return intValue
        }
    }
    return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
    if value, exists := os.LookupEnv(key); exists {
        intValue, err := strconv.ParseInt(value, 10, 64)
        if err == nil {
            return intValue
        }
    }
    return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
    if value, exists := os.LookupEnv(key); exists {
        boolValue, err := strconv.ParseBool(value)
        if err == nil {
            return boolValue
        }
    }
    return defaultValue
}
