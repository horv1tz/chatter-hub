package main

import (
    "fmt"
    "net/http"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
    "github.com/gin-gonic/gin"
    "github.com/go-playground/validator/v10"
    "github.com/golang-jwt/jwt/v4"
    "github.com/gorilla/websocket"
    "github.com/sirupsen/logrus"
    "golang.org/x/crypto/bcrypt"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "golang.org/x/time/rate"
)

// =========================
// Конфигурация
// =========================

// Config хранит конфигурационные параметры
type Config struct {
    DbUser         string
    DbPassword     string
    DbName         string
    DbHost         string
    DbPort         int
    JwtSecret      string
    AwsRegion      string
    S3Bucket       string
    S3Endpoint     string
    S3AccessKey    string
    S3SecretKey    string
    ServerPort     string
    LogLevel       string
    RateLimit      int
    RateLimitBurst int
}

// Загрузка конфигурации из переменных окружения
func loadConfig() Config {
    dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
    if err != nil {
        logrus.Fatalf("Invalid DB_PORT: %v", err)
    }

    rateLimit, err := strconv.Atoi(getEnv("RATE_LIMIT", "10"))
    if err != nil {
        logrus.Fatalf("Invalid RATE_LIMIT: %v", err)
    }

    rateLimitBurst, err := strconv.Atoi(getEnv("RATE_LIMIT_BURST", "20"))
    if err != nil {
        logrus.Fatalf("Invalid RATE_LIMIT_BURST: %v", err)
    }

    return Config{
        DbUser:         getEnv("DB_USER", "youruser"),
        DbPassword:     getEnv("DB_PASSWORD", "yourpassword"),
        DbName:         getEnv("DB_NAME", "yourdb"),
        DbHost:         getEnv("DB_HOST", "localhost"),
        DbPort:         dbPort,
        JwtSecret:      getEnv("JWT_SECRET", "yourjwtsecret"),
        AwsRegion:      getEnv("AWS_REGION", "us-east-1"),
        S3Bucket:       getEnv("S3_BUCKET", "yourbucket"),
        S3Endpoint:     getEnv("S3_ENDPOINT", "http://localhost:9000"),
        S3AccessKey:    getEnv("S3_ACCESS_KEY", "minio"),
        S3SecretKey:    getEnv("S3_SECRET_KEY", "minio123"),
        ServerPort:     getEnv("SERVER_PORT", "8080"),
        LogLevel:       getEnv("LOG_LEVEL", "info"),
        RateLimit:      rateLimit,
        RateLimitBurst: rateLimitBurst,
    }
}

func getEnv(key, defaultValue string) string {
    value, exists := os.LookupEnv(key)
    if !exists {
        return defaultValue
    }
    return value
}

// =========================
// Модели
// =========================

// User представляет пользователя системы
type User struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    Username  string    `json:"username" gorm:"unique;not null"`
    Password  string    `json:"-"` // Не включать в JSON-ответы
    Name      string    `json:"name" gorm:"not null"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// Message представляет сообщение в чате
type Message struct {
    ID         uint      `json:"id" gorm:"primaryKey"`
    ChatID     uint      `json:"chat_id" gorm:"index"` // Добавлено ChatID
    SenderID   uint      `json:"sender_id"`
    ReceiverID uint      `json:"receiver_id"`
    Content    string    `json:"content"`
    MediaURL   string    `json:"media_url,omitempty"`
    Timestamp  time.Time `json:"timestamp"`
    Status     string    `json:"status"` // "sent", "delivered", "read"
}

// Chat представляет чат (личный или групповой)
type Chat struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    Type      string    `json:"type" gorm:"type:varchar(10);not null"` // "personal" или "group"
    Name      string    `json:"name,omitempty"`
    Users     []User    `json:"users" gorm:"many2many:chat_users;"`
    Messages  []Message `json:"messages" gorm:"foreignKey:ChatID"` // Добавлено Messages
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// =========================
// Серверное состояние
// =========================

// Server содержит состояние сервера
type Server struct {
    DB           *gorm.DB
    Config       Config
    Router       *gin.Engine
    Validator    *validator.Validate
    Clients      map[uint]*websocket.Conn
    Notifiers    map[uint]chan Message
    BlockedUsers map[uint]map[uint]bool // userID -> blockedUserID
    Mutex        sync.RWMutex
    Limiter      *rate.Limiter
}

// =========================
// Инициализация
// =========================

func main() {
    // Инициализация конфигурации
    config := loadConfig()

    // Инициализация логирования
    level, err := logrus.ParseLevel(config.LogLevel)
    if err != nil {
        logrus.Fatalf("Invalid log level: %v", err)
    }
    logrus.SetLevel(level)
    logrus.SetFormatter(&logrus.JSONFormatter{})

    // Инициализация базы данных
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        config.DbHost, config.DbPort, config.DbUser, config.DbPassword, config.DbName)
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        logrus.Fatalf("Не удалось подключиться к базе данных: %v", err)
    }

    // Миграция схемы
    err = db.AutoMigrate(&User{}, &Message{}, &Chat{})
    if err != nil {
        logrus.Fatalf("Не удалось выполнить миграцию базы данных: %v", err)
    }

    // Инициализация сервера
    server := &Server{
        DB:           db,
        Config:       config,
        Router:       gin.Default(),
        Validator:    validator.New(),
        Clients:      make(map[uint]*websocket.Conn),
        Notifiers:    make(map[uint]chan Message),
        BlockedUsers: make(map[uint]map[uint]bool),
        Limiter:      rate.NewLimiter(rate.Limit(config.RateLimit), config.RateLimitBurst),
    }

    // Определение WebSocket upgrader
    var upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true // В продакшене настройте более строгую проверку
        },
    }

    // Настройка маршрутов
    server.setupRoutes(upgrader)

    // Запуск сервера
    if err := server.Router.Run(":" + server.Config.ServerPort); err != nil {
        logrus.Fatalf("Не удалось запустить сервер: %v", err)
    }
}

// =========================
// Настройка маршрутов
// =========================

func (s *Server) setupRoutes(upgrader websocket.Upgrader) {
    // Middleware для ограничения скорости
    rateLimitMiddleware := func(c *gin.Context) {
        if !s.Limiter.Allow() {
            s.respondWithError(c, http.StatusTooManyRequests, "Слишком много запросов")
            c.Abort()
            return
        }
        c.Next()
    }

    // Группировка маршрутов с аутентификацией
    api := s.Router.Group("/api", rateLimitMiddleware)
    {
        api.POST("/register", s.Register)
        api.POST("/login", s.Login)
        api.GET("/profile/:user_id", s.AuthMiddleware(), s.GetProfile)
        api.GET("/chats", s.AuthMiddleware(), s.GetChats)
        api.GET("/search", s.AuthMiddleware(), s.SearchUser)
        api.GET("/ws", s.AuthMiddleware(), func(c *gin.Context) {
            s.HandleWebSocket(c, upgrader)
        })
        api.POST("/message", s.AuthMiddleware(), s.SendMessage)
        api.POST("/create-chat", s.AuthMiddleware(), s.CreateChat)
        api.POST("/block-user", s.AuthMiddleware(), s.BlockUser)
        api.POST("/upload", s.AuthMiddleware(), s.UploadFile)
        api.DELETE("/delete-message/:message_id", s.AuthMiddleware(), s.DeleteMessage)
        api.GET("/messages", s.AuthMiddleware(), s.GetMessages) // Эндпоинт для получения сообщений
    }
}

// =========================
// Middleware
// =========================

// AuthMiddleware проверяет JWT-токен
func (s *Server) AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        logrus.Infof("Received Authorization header: %s", authHeader) // Добавлено для отладки

        if authHeader == "" {
            s.respondWithError(c, http.StatusUnauthorized, "Authorization header missing")
            c.Abort()
            return
        }

        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            s.respondWithError(c, http.StatusUnauthorized, "Invalid Authorization header format")
            c.Abort()
            return
        }

        tokenStr := parts[1]

        token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
            // Проверка метода подписи
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method")
            }
            return []byte(s.Config.JwtSecret), nil
        })

        if err != nil || !token.Valid {
            s.respondWithError(c, http.StatusUnauthorized, "Invalid or expired token")
            c.Abort()
            return
        }

        claims, ok := token.Claims.(jwt.MapClaims)
        if !ok {
            s.respondWithError(c, http.StatusUnauthorized, "Invalid token claims")
            c.Abort()
            return
        }

        userIDFloat, ok := claims["user_id"].(float64)
        if !ok {
            s.respondWithError(c, http.StatusUnauthorized, "Invalid user ID in token")
            c.Abort()
            return
        }

        userID := uint(userIDFloat)
        c.Set("user_id", userID)
        c.Next()
    }
}

// =========================
// Обработчики
// =========================

// Register обрабатывает регистрацию пользователя
func (s *Server) Register(c *gin.Context) {
    var input struct {
        Username string `json:"username" validate:"required,min=3,max=32"`
        Password string `json:"password" validate:"required,min=6"`
        Name     string `json:"name" validate:"required"`
    }

    if err := c.ShouldBindJSON(&input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Неверный формат данных")
        return
    }

    // Валидация данных
    if err := s.Validator.Struct(input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, err.Error())
        return
    }

    // Проверка уникальности username
    var existingUser User
    if err := s.DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
        s.respondWithError(c, http.StatusBadRequest, "Имя пользователя уже занято")
        return
    }

    // Хеширование пароля
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
    if err != nil {
        logrus.Errorf("Ошибка при хешировании пароля: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Внутренняя ошибка сервера")
        return
    }

    // Создание пользователя
    user := User{
        Username: input.Username,
        Password: string(hashedPassword),
        Name:     input.Name,
    }

    if err := s.DB.Create(&user).Error; err != nil {
        logrus.Errorf("Ошибка при создании пользователя: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось зарегистрировать пользователя")
        return
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{"message": "Пользователь успешно зарегистрирован"})
}

// Login обрабатывает аутентификацию пользователя
func (s *Server) Login(c *gin.Context) {
    var input struct {
        Username string `json:"username" validate:"required"`
        Password string `json:"password" validate:"required"`
    }

    if err := c.ShouldBindJSON(&input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Неверный формат данных")
        return
    }

    // Валидация данных
    if err := s.Validator.Struct(input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, err.Error())
        return
    }

    // Поиск пользователя
    var user User
    if err := s.DB.Where("username = ?", input.Username).First(&user).Error; err != nil {
        s.respondWithError(c, http.StatusUnauthorized, "Неверные учетные данные")
        return
    }

    // Сравнение паролей
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
        s.respondWithError(c, http.StatusUnauthorized, "Неверные учетные данные")
        return
    }

    // Генерация JWT
    token, err := s.generateJWT(user.ID)
    if err != nil {
        logrus.Errorf("Ошибка при генерации JWT: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось сгенерировать токен")
        return
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{
        "message": "Успешный вход",
        "token":   token,
    })
}

// GetProfile возвращает профиль пользователя
func (s *Server) GetProfile(c *gin.Context) {
    userIDParam := c.Param("user_id")
    requestedUserID, err := strconv.ParseUint(userIDParam, 10, 64)
    if err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Неверный ID пользователя")
        return
    }

    var user User
    if err := s.DB.First(&user, requestedUserID).Error; err != nil {
        s.respondWithError(c, http.StatusNotFound, "Пользователь не найден")
        return
    }

    // Исключение пароля из ответа
    user.Password = ""

    s.respondWithJSON(c, http.StatusOK, gin.H{"user": user})
}

// GetChats возвращает список чатов пользователя
func (s *Server) GetChats(c *gin.Context) {
    userID := c.GetUint("user_id")

    var chats []Chat
    if err := s.DB.
        Joins("JOIN chat_users ON chat_users.chat_id = chats.id").
        Where("chat_users.user_id = ?", userID).
        Preload("Users").
        Preload("Messages").
        Find(&chats).Error; err != nil {
        logrus.Errorf("Ошибка при получении чатов: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось получить чаты")
        return
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{"chats": chats})
}

// SearchUser ищет пользователей по имени пользователя
func (s *Server) SearchUser(c *gin.Context) {
    username := c.Query("username")
    if username == "" {
        s.respondWithError(c, http.StatusBadRequest, "Параметр 'username' обязателен")
        return
    }

    var users []User
    if err := s.DB.Where("username ILIKE ?", "%"+username+"%").Find(&users).Error; err != nil {
        logrus.Errorf("Ошибка при поиске пользователей: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось найти пользователей")
        return
    }

    // Исключение паролей из ответа
    for i := range users {
        users[i].Password = ""
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{"users": users})
}

// HandleWebSocket обрабатывает WebSocket подключения
func (s *Server) HandleWebSocket(c *gin.Context, upgrader websocket.Upgrader) {
    userID := c.GetUint("user_id")

    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        logrus.Errorf("Ошибка при обновлении WebSocket: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось установить WebSocket соединение")
        return
    }
    // Не используйте defer conn.Close() здесь, так как функция блокируется до закрытия соединения

    // Регистрация клиента
    s.registerClient(userID, conn)
    // Удаление клиента произойдет внутри unregisterClient при закрытии соединения

    // Чтение сообщений от клиента
    for {
        var msg Message
        err := conn.ReadJSON(&msg)
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                logrus.Errorf("Неожиданное закрытие WebSocket для пользователя %d: %v", userID, err)
            } else {
                logrus.Infof("WebSocket соединение для пользователя %d закрыто: %v", userID, err)
            }
            break
        }

        // Обработка входящего сообщения
        go s.processIncomingMessage(userID, &msg)
    }

    // Отключение клиента
    s.unregisterClient(userID)
}

// SendMessage обрабатывает отправку сообщений через HTTP
func (s *Server) SendMessage(c *gin.Context) {
    var input struct {
        ReceiverID uint   `json:"receiver_id" validate:"required"`
        Content    string `json:"content" validate:"required"`
        MediaURL   string `json:"media_url"`
    }

    if err := c.ShouldBindJSON(&input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Неверный формат данных")
        return
    }

    // Валидация данных
    if err := s.Validator.Struct(input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, err.Error())
        return
    }

    senderID := c.GetUint("user_id")

    // Проверка блокировок
    if s.isBlocked(input.ReceiverID, senderID) {
        s.respondWithError(c, http.StatusForbidden, "Вы заблокированы этим пользователем")
        return
    }

    // Найти или создать личный чат между sender и receiver
    chat, err := s.findOrCreatePersonalChat(senderID, input.ReceiverID)
    if err != nil {
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось найти или создать чат")
        return
    }

    // Создание сообщения
    msg := Message{
        ChatID:     chat.ID,
        SenderID:   senderID,
        ReceiverID: input.ReceiverID,
        Content:    input.Content,
        MediaURL:   input.MediaURL,
        Timestamp:  time.Now(),
        Status:     "sent",
    }

    // Сохранение в базу данных
    if err := s.DB.Create(&msg).Error; err != nil {
        logrus.Errorf("Ошибка при сохранении сообщения: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось отправить сообщение")
        return
    }

    // Уведомление получателя через WebSocket
    s.notifyReceiver(input.ReceiverID, msg)

    // Возвращаем message_id
    s.respondWithJSON(c, http.StatusOK, gin.H{"message": "Сообщение отправлено", "message_id": msg.ID})
}

// CreateChat обрабатывает создание нового чата
func (s *Server) CreateChat(c *gin.Context) {
    var input struct {
        Type    string `json:"type" validate:"required,oneof=personal group"`
        Name    string `json:"name"` // Опционально для групповых чатов
        UserIDs []uint `json:"user_ids" validate:"required,min=1,dive,required"`
    }

    if err := c.ShouldBindJSON(&input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Неверный формат данных")
        return
    }

    // Валидация данных
    if err := s.Validator.Struct(input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, err.Error())
        return
    }

    // Проверка количества пользователей для личного чата
    if input.Type == "personal" && len(input.UserIDs) != 2 {
        s.respondWithError(c, http.StatusBadRequest, "Личный чат должен содержать ровно двух пользователей")
        return
    }

    // Проверка существования пользователей
    var users []User
    for _, userID := range input.UserIDs {
        var user User
        if err := s.DB.First(&user, userID).Error; err != nil {
            s.respondWithError(c, http.StatusBadRequest, fmt.Sprintf("Пользователь с ID %d не найден", userID))
            return
        }
        users = append(users, user)
    }

    // Создание чата
    chat := Chat{
        Type:  input.Type,
        Name:  input.Name,
        Users: users,
    }

    if err := s.DB.Create(&chat).Error; err != nil {
        logrus.Errorf("Ошибка при создании чата: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось создать чат")
        return
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{"message": "Чат создан", "chat": chat})
}

// BlockUser обрабатывает блокировку пользователя
func (s *Server) BlockUser(c *gin.Context) {
    var input struct {
        BlockID uint `json:"block_id" validate:"required"`
    }

    if err := c.ShouldBindJSON(&input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Неверный формат данных")
        return
    }

    // Валидация данных
    if err := s.Validator.Struct(input); err != nil {
        s.respondWithError(c, http.StatusBadRequest, err.Error())
        return
    }

    userID := c.GetUint("user_id")

    // Проверка существования пользователя для блокировки
    var userToBlock User
    if err := s.DB.First(&userToBlock, input.BlockID).Error; err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Пользователь для блокировки не найден")
        return
    }

    // Блокировка пользователя
    s.Mutex.Lock()
    defer s.Mutex.Unlock()

    if _, exists := s.BlockedUsers[userID]; !exists {
        s.BlockedUsers[userID] = make(map[uint]bool)
    }
    s.BlockedUsers[userID][input.BlockID] = true

    s.respondWithJSON(c, http.StatusOK, gin.H{"message": "Пользователь заблокирован"})
}

// UploadFile обрабатывает загрузку файлов в S3
func (s *Server) UploadFile(c *gin.Context) {
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Ошибка при загрузке файла")
        return
    }
    defer file.Close()

    // Инициализация S3 сессии
    sess, err := session.NewSession(&aws.Config{
        Region:           aws.String(s.Config.AwsRegion),
        Endpoint:         aws.String(s.Config.S3Endpoint),
        S3ForcePathStyle: aws.Bool(true),
        Credentials:      credentials.NewStaticCredentials(s.Config.S3AccessKey, s.Config.S3SecretKey, ""),
    })
    if err != nil {
        logrus.Errorf("Ошибка при создании S3 сессии: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось создать S3 сессию")
        return
    }

    uploader := s3.New(sess)

    // Генерация уникального имени файла
    fileName := fmt.Sprintf("%d-%s", time.Now().Unix(), sanitizeFileName(header.Filename))

    // Загрузка файла в S3
    _, err = uploader.PutObject(&s3.PutObjectInput{
        Bucket: aws.String(s.Config.S3Bucket),
        Key:    aws.String(fileName),
        Body:   file,
        ACL:    aws.String("public-read"),
    })
    if err != nil {
        logrus.Errorf("Ошибка при загрузке файла в S3: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось загрузить файл")
        return
    }

    // Формирование URL файла
    var fileURL string
    if strings.Contains(s.Config.S3Endpoint, "amazonaws.com") {
        // Стандартный AWS S3 URL
        fileURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.Config.S3Bucket, s.Config.AwsRegion, fileName)
    } else {
        // Пользовательский S3-совместимый сервис (например, MinIO)
        fileURL = fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.Config.S3Endpoint, "/"), s.Config.S3Bucket, fileName)
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{
        "message":  "Файл успешно загружен",
        "file_url": fileURL,
    })
}

// DeleteMessage обрабатывает удаление сообщения
func (s *Server) DeleteMessage(c *gin.Context) {
    messageIDStr := c.Param("message_id")
    messageID, err := strconv.ParseUint(messageIDStr, 10, 64)
    if err != nil {
        s.respondWithError(c, http.StatusBadRequest, "Неверный ID сообщения")
        return
    }

    var msg Message
    if err := s.DB.First(&msg, messageID).Error; err != nil {
        s.respondWithError(c, http.StatusNotFound, "Сообщение не найдено")
        return
    }

    userID := c.GetUint("user_id")
    if msg.SenderID != userID {
        s.respondWithError(c, http.StatusForbidden, "Вы не можете удалить это сообщение")
        return
    }

    if err := s.DB.Delete(&msg).Error; err != nil {
        logrus.Errorf("Ошибка при удалении сообщения: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось удалить сообщение")
        return
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{"message": "Сообщение успешно удалено"})
}

// GetMessages возвращает список сообщений пользователя
func (s *Server) GetMessages(c *gin.Context) {
    userID := c.GetUint("user_id")

    var messages []Message
    if err := s.DB.Where("sender_id = ? OR receiver_id = ?", userID, userID).
        Preload("Chat").
        Find(&messages).Error; err != nil {
        logrus.Errorf("Ошибка при получении сообщений: %v", err)
        s.respondWithError(c, http.StatusInternalServerError, "Не удалось получить сообщения")
        return
    }

    s.respondWithJSON(c, http.StatusOK, gin.H{"messages": messages})
}

// =========================
// Вспомогательные функции
// =========================

// respondWithError отправляет ошибку в JSON формате
func (s *Server) respondWithError(c *gin.Context, code int, message string) {
    s.respondWithJSON(c, code, gin.H{"error": message})
}

// respondWithJSON отправляет данные в JSON формате
func (s *Server) respondWithJSON(c *gin.Context, code int, payload interface{}) {
    c.JSON(code, payload)
}

// sanitizeFileName удаляет потенциально опасные символы из имени файла
func sanitizeFileName(name string) string {
    // Дополнительная очистка имени файла
    name = strings.ReplaceAll(name, " ", "_")
    name = strings.Map(func(r rune) rune {
        if r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
            return r
        }
        return -1
    }, name)
    return name
}

// generateJWT генерирует JWT-токен для пользователя
func (s *Server) generateJWT(userID uint) (string, error) {
    claims := jwt.MapClaims{
        "user_id": userID,
        "exp":     time.Now().Add(time.Hour * 72).Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(s.Config.JwtSecret))
}

// =========================
// Управление клиентами WebSocket
// =========================

// registerClient регистрирует нового клиента
func (s *Server) registerClient(userID uint, conn *websocket.Conn) {
    s.Mutex.Lock()
    defer s.Mutex.Unlock()

    s.Clients[userID] = conn
    if _, exists := s.Notifiers[userID]; !exists {
        s.Notifiers[userID] = make(chan Message, 100) // Буферизованный канал
        go s.listenToNotifier(userID)
    }

    logrus.Infof("Пользователь %d подключен через WebSocket", userID)
}

// unregisterClient удаляет клиента из списка подключенных
func (s *Server) unregisterClient(userID uint) {
    s.Mutex.Lock()
    defer s.Mutex.Unlock()

    if conn, exists := s.Clients[userID]; exists {
        conn.Close()
        delete(s.Clients, userID)
    }

    if notifier, exists := s.Notifiers[userID]; exists {
        close(notifier)
        delete(s.Notifiers, userID)
    }

    logrus.Infof("Пользователь %d отключен от WebSocket", userID)
}

// listenToNotifier слушает канал уведомлений и отправляет сообщения клиенту
func (s *Server) listenToNotifier(userID uint) {
    for msg := range s.Notifiers[userID] {
        s.Mutex.RLock()
        conn, exists := s.Clients[userID]
        s.Mutex.RUnlock()
        if exists {
            err := conn.WriteJSON(msg)
            if err != nil {
                logrus.Errorf("Ошибка при отправке сообщения пользователю %d: %v", userID, err)
                s.unregisterClient(userID)
                break
            }
        }
    }
}

// processIncomingMessage обрабатывает входящие сообщения через WebSocket
func (s *Server) processIncomingMessage(senderID uint, msg *Message) {
    // Проверка блокировок
    if s.isBlocked(msg.ReceiverID, senderID) {
        logrus.Infof("Пользователь %d заблокирован пользователем %d", senderID, msg.ReceiverID)
        return
    }

    // Найти или создать чат между отправителем и получателем
    chat, err := s.findOrCreatePersonalChat(senderID, msg.ReceiverID)
    if err != nil {
        logrus.Errorf("Не удалось найти или создать чат: %v", err)
        return
    }

    // Установка свойств сообщения
    msg.ChatID = chat.ID
    msg.SenderID = senderID
    msg.Timestamp = time.Now()
    msg.Status = "sent"

    // Сохранение в базу данных
    if err := s.DB.Create(msg).Error; err != nil {
        logrus.Errorf("Ошибка при сохранении сообщения: %v", err)
        return
    }

    // Уведомление получателя через WebSocket
    s.notifyReceiver(msg.ReceiverID, *msg)
}

// notifyReceiver отправляет сообщение получателю через WebSocket, если он подключен
func (s *Server) notifyReceiver(receiverID uint, msg Message) {
    s.Mutex.RLock()
    notifier, exists := s.Notifiers[receiverID]
    s.Mutex.RUnlock()
    if exists {
        select {
        case notifier <- msg:
            logrus.Infof("Сообщение отправлено пользователю %d через WebSocket", receiverID)
        default:
            logrus.Warnf("Буфер уведомлений пользователя %d переполнен", receiverID)
        }
    }
}

// isBlocked проверяет, заблокирован ли отправитель получателем
func (s *Server) isBlocked(receiverID, senderID uint) bool {
    s.Mutex.RLock()
    defer s.Mutex.RUnlock()

    if blocked, exists := s.BlockedUsers[receiverID]; exists {
        return blocked[senderID]
    }
    return false
}

// findOrCreatePersonalChat ищет личный чат между двумя пользователями или создаёт его
func (s *Server) findOrCreatePersonalChat(user1ID, user2ID uint) (*Chat, error) {
    var chat Chat
    err := s.DB.
        Joins("JOIN chat_users cu1 ON cu1.chat_id = chats.id").
        Joins("JOIN chat_users cu2 ON cu2.chat_id = chats.id").
        Where("chats.type = ? AND cu1.user_id = ? AND cu2.user_id = ?", "personal", user1ID, user2ID).
        Preload("Users").
        First(&chat).Error

    if err != nil {
        if err == gorm.ErrRecordNotFound {
            // Создать личный чат
            chat = Chat{
                Type: "personal",
            }
            // Найти отправителя и получателя
            var sender, receiver User
            if err := s.DB.First(&sender, user1ID).Error; err != nil {
                return nil, fmt.Errorf("не удалось найти отправителя: %v", err)
            }
            if err := s.DB.First(&receiver, user2ID).Error; err != nil {
                return nil, fmt.Errorf("не удалось найти получателя: %v", err)
            }
            chat.Users = []User{sender, receiver}
            if err := s.DB.Create(&chat).Error; err != nil {
                return nil, fmt.Errorf("ошибка при создании личного чата: %v", err)
            }
        } else {
            return nil, fmt.Errorf("ошибка при поиске чата: %v", err)
        }
    }

    return &chat, nil
}
