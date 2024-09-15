package config

import (
    "fmt"
    "log"
    "time"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

// Объявление модели User
type User struct {
    ID       string `gorm:"primaryKey" json:"id" example:"12345"`
    Username string `json:"username" example:"john_doe"`
    Email    string `json:"email" example:"john@example.com"`
    Password string `json:"password,omitempty" example:"secret"`
    IsActive bool   `json:"is_active" gorm:"default:true"` // Новое поле
}

// Объявление модели TextMessage
type TextMessage struct {
    ID         uint      `gorm:"primaryKey" json:"id"`
    SenderID   string    `json:"sender_id"`
    ReceiverID string    `json:"receiver_id"`
    Content    string    `json:"content"`
    CreatedAt  time.Time `json:"created_at"`
}

// Объявление модели VoiceMessage
type VoiceMessage struct {
    ID         uint      `gorm:"primaryKey" json:"id"`
    SenderID   string    `json:"sender_id"`
    ReceiverID string    `json:"receiver_id"`
    FileURL    string    `json:"file_url"`
    CreatedAt  time.Time `json:"created_at"`
}

var DB *gorm.DB

// InitDB инициализирует соединение с базой данных PostgreSQL
func InitDB(cfg *Config) {
    dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.DBName)

    var err error
    DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatalf("Ошибка подключения к базе данных: %v", err)
    }

    // Автоматическая миграция схемы
    if err := DB.AutoMigrate(&User{}, &TextMessage{}, &VoiceMessage{}); err != nil {
        log.Fatalf("Ошибка миграции базы данных: %v", err)
    }
}
