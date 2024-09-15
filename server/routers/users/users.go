package users

import (
    "net/http"

    "chatter-hub-server/auth"
    "chatter-hub-server/config"

    "github.com/gin-gonic/gin"
    "golang.org/x/crypto/bcrypt"
    "github.com/google/uuid"
)

// CreateUser godoc
//	@Summary		Создание пользователя
//	@Description	Создает нового пользователя и возвращает JWT токен
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			user	body		config.User			true	"Информация о пользователе"
//	@Success		200		{object}	map[string]string	"token"	"JWT токен"
//	@Failure		400		{object}	config.ErrorResponse
//	@Failure		500		{object}	config.ErrorResponse
//	@Router			/users [post]
func CreateUser(c *gin.Context) {
    var user config.User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(http.StatusBadRequest, config.ErrorResponse{Error: err.Error()})
        return
    }

    // Хешируем пароль
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка хеширования пароля"})
        return
    }
    user.Password = string(hashedPassword)

    // Генерируем уникальный ID
    user.ID = uuid.New().String()

    // Сохраняем пользователя в базе данных
    if err := config.DB.Create(&user).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка создания пользователя"})
        return
    }

    // Загружаем конфигурацию
    cfg, err := config.LoadConfig()
    if err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка загрузки конфигурации"})
        return
    }

    // Генерируем JWT токен
    token, err := auth.GenerateToken(user.ID, cfg)
    if err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка создания JWT токена"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"token": token})
}


// GetUser godoc
//	@Summary		Получение информации о пользователе
//	@Description	Возвращает информацию о пользователе по ID
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"ID пользователя"
//	@Success		200	{object}	config.User
//	@Failure		404	{object}	config.ErrorResponse
//	@Failure		500	{object}	config.ErrorResponse
//	@Router			/users/{id} [get]
func GetUser(c *gin.Context) {
    userID := c.Param("id")
    var user config.User

    // Проверяем кэш Redis
    val, err := config.RedisClient.Get(config.Ctx, "user:"+userID).Result()
    if err == nil {
        // Пользователь найден в кэше
        c.JSON(http.StatusOK, val)
        return
    }

    if err := config.DB.First(&user, "id = ?", userID).Error; err != nil {
        c.JSON(http.StatusNotFound, config.ErrorResponse{Error: "Пользователь не найден"})
        return
    }

    // Кэшируем пользователя в Redis
    config.RedisClient.Set(config.Ctx, "user:"+userID, user, 0)

    c.JSON(http.StatusOK, user)
}

// UpdateUser godoc
//	@Summary		Обновление информации о пользователе
//	@Description	Обновляет информацию о пользователе по ID
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string		true	"ID пользователя"
//	@Param			user	body		config.User	true	"Обновленная информация о пользователе"
//	@Success		200		{object}	config.User
//	@Failure		400		{object}	config.ErrorResponse
//	@Failure		500		{object}	config.ErrorResponse
//	@Router			/users/{id} [put]
func UpdateUser(c *gin.Context) {
    userID := c.Param("id")
    var user config.User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(http.StatusBadRequest, config.ErrorResponse{Error: err.Error()})
        return
    }

    // Проверяем, совпадает ли ID пользователя с тем, который в токене
    tokenUserID := c.GetString("userID")
    if tokenUserID != userID {
        c.JSON(http.StatusForbidden, config.ErrorResponse{Error: "Нет прав для изменения этого пользователя"})
        return
    }

    // Хешируем пароль, если он изменяется
    if user.Password != "" {
        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
        if err != nil {
            c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка хеширования пароля"})
            return
        }
        user.Password = string(hashedPassword)
    }

    // Обновляем пользователя в базе данных
    if err := config.DB.Model(&config.User{}).Where("id = ?", userID).Updates(user).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка обновления пользователя"})
        return
    }

    // Удаляем пользователя из кэша Redis
    config.RedisClient.Del(config.Ctx, "user:"+userID)

    c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
//	@Summary		Удаление пользователя
//	@Description	Удаляет пользователя по ID
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"ID пользователя"
//	@Success		200	{object}	config.SimpleResponse
//	@Failure		500	{object}	config.ErrorResponse
//	@Router			/users/{id} [delete]
func DeleteUser(c *gin.Context) {
    userID := c.Param("id")

    // Проверяем, совпадает ли ID пользователя с тем, который в токене
    tokenUserID := c.GetString("userID")
    if tokenUserID != userID {
        c.JSON(http.StatusForbidden, config.ErrorResponse{Error: "Нет прав для удаления этого пользователя"})
        return
    }

    // Удаляем пользователя из базы данных
    if err := config.DB.Delete(&config.User{}, "id = ?", userID).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка удаления пользователя"})
        return
    }

    // Удаляем пользователя из кэша Redis
    config.RedisClient.Del(config.Ctx, "user:"+userID)

    c.JSON(http.StatusOK, config.SimpleResponse{Message: "Пользователь удален"})
}


// LoginUser godoc
// @Summary      Аутентификация пользователя
// @Description  Аутентифицирует пользователя и возвращает JWT токен
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        credentials  body      LoginRequest  true  "Учетные данные пользователя"
// @Success      200          {object}  TokenResponse
// @Failure      400          {object}  config.ErrorResponse
// @Failure      401          {object}  config.ErrorResponse
// @Failure      500          {object}  config.ErrorResponse
// @Router       /login [post]
func LoginUser(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, config.ErrorResponse{Error: err.Error()})
        return
    }

    var user config.User
    // Поиск пользователя по email или username
    if req.Email != "" {
        if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
            c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Неверные учетные данные"})
            return
        }
    } else if req.Username != "" {
        if err := config.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
            c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Неверные учетные данные"})
            return
        }
    } else {
        c.JSON(http.StatusBadRequest, config.ErrorResponse{Error: "Необходимо указать email или username"})
        return
    }

    // Проверка пароля
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
        c.JSON(http.StatusUnauthorized, config.ErrorResponse{Error: "Неверные учетные данные"})
        return
    }

    // Проверка, активен ли аккаунт
    if !user.IsActive {
        c.JSON(http.StatusForbidden, config.ErrorResponse{Error: "Аккаунт деактивирован"})
        return
    }

    // Генерируем JWT токен
    cfg, err := config.LoadConfig()
    if err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка загрузки конфигурации"})
        return
    }

    token, err := auth.GenerateToken(user.ID, cfg)
    if err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка создания JWT токена"})
        return
    }

    c.JSON(http.StatusOK, TokenResponse{Token: token})
}

// LoginRequest представляет запрос на аутентификацию
type LoginRequest struct {
    Email    string `json:"email,omitempty"`
    Username string `json:"username,omitempty"`
    Password string `json:"password" binding:"required"`
}

// TokenResponse представляет ответ с JWT токеном
type TokenResponse struct {
    Token string `json:"token"`
}

// DeactivateUser godoc
// @Summary      Деактивация пользователя
// @Description  Деактивирует учетную запись пользователя по ID
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "ID пользователя"
// @Success      200   {object}  config.SimpleResponse
// @Failure      500   {object}  config.ErrorResponse
// @Router       /users/{id}/deactivate [post]
func DeactivateUser(c *gin.Context) {
    userID := c.Param("id")

    // Проверяем, совпадает ли ID пользователя с тем, который в токене
    tokenUserID := c.GetString("userID")
    if tokenUserID != userID {
        c.JSON(http.StatusForbidden, config.ErrorResponse{Error: "Нет прав для деактивации этого пользователя"})
        return
    }

    // Обновляем поле IsActive в базе данных
    if err := config.DB.Model(&config.User{}).Where("id = ?", userID).Update("is_active", false).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка деактивации пользователя"})
        return
    }

    c.JSON(http.StatusOK, config.SimpleResponse{Message: "Пользователь деактивирован"})
}

// ActivateUser godoc
// @Summary      Активация пользователя
// @Description  Активирует учетную запись пользователя по ID
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "ID пользователя"
// @Success      200   {object}  config.SimpleResponse
// @Failure      500   {object}  config.ErrorResponse
// @Router       /users/{id}/activate [post]
func ActivateUser(c *gin.Context) {
    userID := c.Param("id")

    // Проверяем, совпадает ли ID пользователя с тем, который в токене
    tokenUserID := c.GetString("userID")
    if tokenUserID != userID {
        c.JSON(http.StatusForbidden, config.ErrorResponse{Error: "Нет прав для активации этого пользователя"})
        return
    }

    // Обновляем поле IsActive в базе данных
    if err := config.DB.Model(&config.User{}).Where("id = ?", userID).Update("is_active", true).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка активации пользователя"})
        return
    }

    c.JSON(http.StatusOK, config.SimpleResponse{Message: "Пользователь активирован"})
}
