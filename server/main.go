package main

import (
    "fmt"
    "log"

    "chatter-hub-server/auth"
    "chatter-hub-server/config"
    "chatter-hub-server/routers"

    _ "chatter-hub-server/docs" // Это нужно для загрузки сгенерированных файлов Swagger

    "github.com/gin-gonic/gin"
    ginSwagger "github.com/swaggo/gin-swagger"
    "github.com/swaggo/files"
	"chatter-hub-server/routers/users" // Добавьте этот импорт
	
)

// @title           Messenger API
// @version         1.0
// @description     API для мессенджера.

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
    // Загружаем конфигурацию
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Ошибка загрузки конфигурации: %v", err)
    }

    // Инициализируем базу данных
    config.InitDB(cfg)

    // Инициализируем Redis
    config.InitRedis(cfg)

    // Инициализируем MinIO
    config.InitMinio(cfg)

    // Инициализируем роутер Gin
    router := gin.Default()

    // Незапрашиваемые маршруты
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.POST("/users", users.CreateUser)  // Создание пользователя (не защищено)
	router.POST("/login", users.LoginUser)   // Аутентификация (не защищено)

    // Защищенные маршруты
    authorized := router.Group("/")
    authorized.Use(auth.AuthMiddleware(cfg)) // Middleware аутентификации для защищенных маршрутов
    {
        routers.RegisterProtectedRoutes(authorized)
    }

    // Запуск сервера
    address := fmt.Sprintf("%s:%s", cfg.API.Host, cfg.API.Port)
    if err := router.Run(address); err != nil {
        log.Fatalf("Ошибка запуска сервера: %v", err)
    }
}
