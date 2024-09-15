package auth

import (
    "time"
    "github.com/dgrijalva/jwt-go"
    "chatter-hub-server/config"
    "github.com/gin-gonic/gin"
    "net/http"
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
        return "", err
    }
    return tokenString, nil
}

// AuthMiddleware middleware для проверки JWT токена
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString := c.GetHeader("Authorization")
        if tokenString == "" {
            c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Необходим токен авторизации"})
            c.Abort()
            return
        }

        claims := &Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return []byte(cfg.JWT.SecretKey), nil
        })

        if err != nil || !token.Valid {
            c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Недействительный токен"})
            c.Abort()
            return
        }

        // Сохраняем идентификатор пользователя в контексте
        c.Set("userID", claims.UserID)
        c.Next()
    }
}
