package auth

import (
    "strings"
    "time"
    "github.com/golang-jwt/jwt/v4" // Используем новую версию библиотеки JWT
    "chatter-hub-server/config"
    "github.com/gin-gonic/gin"
    "net/http"
    "log"
)

// Claims представляет утверждения для токена JWT
type Claims struct {
    UserID string `json:"user_id"`
    jwt.StandardClaims
}

// GenerateToken создает JWT токен для пользователя
func GenerateToken(userID string, cfg *config.Config) (string, error) {
    expirationTime := time.Now().Add(time.Duration(cfg.JWT.ExpiresIn) * time.Second)
    claims := &Claims{
        UserID: userID,
        StandardClaims: jwt.StandardClaims{
            ExpiresAt: expirationTime.Unix(),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(cfg.JWT.SecretKey))
    if err != nil {
        log.Printf("Ошибка создания JWT токена: %v", err)
        return "", err
    }
    log.Printf("Создан JWT токен для пользователя %s: %s", userID, tokenString)
    return tokenString, nil
}

// AuthMiddleware middleware для проверки JWT токена
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Необходим токен авторизации"})
            c.Abort()
            return
        }

        // Ожидаем, что заголовок имеет формат "Bearer <token>"
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || parts[0] != "Bearer" {
            log.Println("Недействительный формат токена авторизации")
            c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Недействительный токен"})
            c.Abort()
            return
        }

        tokenString := parts[1]
        log.Printf("Получен токен: %s", tokenString)

        claims := &Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return []byte(cfg.JWT.SecretKey), nil
        })

        if err != nil || !token.Valid {
            log.Printf("Ошибка проверки токена: %v", err)
            c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Недействительный токен"})
            c.Abort()
            return
        }

        log.Printf("Токен действителен. UserID: %s", claims.UserID)

        // Сохраняем идентификатор пользователя в контексте
        c.Set("userID", claims.UserID)
        c.Next()
    }
}
