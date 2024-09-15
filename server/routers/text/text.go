package text

import (
    "net/http"
    "time"

    "chatter-hub-server/config"

    "github.com/gin-gonic/gin"
)

// SendTextMessage godoc
//	@Summary		Отправка текстового сообщения
//	@Description	Отправляет текстовое сообщение от одного пользователя к другому
//	@Tags			text
//	@Accept			json
//	@Produce		json
//	@Param			message	body		config.TextMessage	true	"Текстовое сообщение"
//	@Success		200		{object}	config.SimpleResponse
//	@Failure		400		{object}	config.ErrorResponse
//	@Failure		500		{object}	config.ErrorResponse
//	@Router			/messages/text [post]
func SendTextMessage(c *gin.Context) {
    var message config.TextMessage
    if err := c.ShouldBindJSON(&message); err != nil {
        c.JSON(http.StatusBadRequest, config.ErrorResponse{Error: err.Error()})
        return
    }

    // Получаем ID пользователя из контекста (из токена)
    senderID := c.GetString("userID")
    message.SenderID = senderID
    message.CreatedAt = time.Now()

    // Сохраняем сообщение в базе данных
    if err := config.DB.Create(&message).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка отправки сообщения"})
        return
    }

    c.JSON(http.StatusOK, config.SimpleResponse{Message: "Текстовое сообщение отправлено"})
}

// GetTextMessages godoc
//	@Summary		Получение текстовых сообщений
//	@Description	Возвращает список текстовых сообщений между двумя пользователями
//	@Tags			text
//	@Accept			json
//	@Produce		json
//	@Param			sender_id	query		string	true	"ID отправителя"
//	@Param			receiver_id	query		string	true	"ID получателя"
//	@Success		200			{array}		config.TextMessage
//	@Failure		400			{object}	config.ErrorResponse
//	@Failure		500			{object}	config.ErrorResponse
//	@Router			/messages/text [get]
func GetTextMessages(c *gin.Context) {
    senderID := c.Query("sender_id")
    receiverID := c.Query("receiver_id")

    var messages []config.TextMessage

    // Проверяем права доступа
    tokenUserID := c.GetString("userID")
    if tokenUserID != senderID && tokenUserID != receiverID {
        c.JSON(http.StatusForbidden, config.ErrorResponse{Error: "Нет прав для просмотра сообщений"})
        return
    }

    // Получаем сообщения из базы данных
    if err := config.DB.Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
        senderID, receiverID, receiverID, senderID).Order("created_at asc").Find(&messages).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка получения сообщений"})
        return
    }

    c.JSON(http.StatusOK, messages)
}
