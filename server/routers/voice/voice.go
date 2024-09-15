package voice

import (
    "fmt"
    "net/http"
    "time"

    "chatter-hub-server/config"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/minio/minio-go/v7"
)

// SendVoiceMessage godoc
//	@Summary		Отправка голосового сообщения
//	@Description	Отправляет голосовое сообщение от одного пользователя к другому
//	@Tags			voice
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			sender_id	formData	string	true	"ID отправителя"
//	@Param			receiver_id	formData	string	true	"ID получателя"
//	@Param			file		formData	file	true	"Аудиофайл"
//	@Success		200			{object}	config.SimpleResponse
//	@Failure		400			{object}	config.ErrorResponse
//	@Failure		500			{object}	config.ErrorResponse
//	@Router			/messages/voice [post]
func SendVoiceMessage(c *gin.Context) {
    receiverID := c.PostForm("receiver_id")

    // Получаем ID пользователя из контекста (из токена)
    senderID := c.GetString("userID")

    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, config.ErrorResponse{Error: "Файл обязателен"})
        return
    }

    // Открываем файл
    fileContent, err := file.Open()
    if err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка открытия файла"})
        return
    }
    defer fileContent.Close()

    // Генерируем уникальное имя файла
    fileName := uuid.New().String() + "-" + file.Filename

    // Загрузка файла в MinIO
    bucketName := "voice-messages"
    _, err = config.MinioClient.PutObject(config.Ctx, bucketName, fileName, fileContent, file.Size, minio.PutObjectOptions{
        ContentType: file.Header.Get("Content-Type"),
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка загрузки файла"})
        return
    }

    // Генерируем URL для доступа к файлу
    fileURL := fmt.Sprintf("http://%s/%s/%s", config.MinioClient.EndpointURL().Host, bucketName, fileName)

    message := config.VoiceMessage{
        SenderID:   senderID,
        ReceiverID: receiverID,
        FileURL:    fileURL,
        CreatedAt:  time.Now(),
    }

    // Сохраняем сообщение в базе данных
    if err := config.DB.Create(&message).Error; err != nil {
        c.JSON(http.StatusInternalServerError, config.ErrorResponse{Error: "Ошибка отправки сообщения"})
        return
    }

    c.JSON(http.StatusOK, config.SimpleResponse{Message: "Голосовое сообщение отправлено"})
}

// GetVoiceMessages godoc
//	@Summary		Получение голосовых сообщений
//	@Description	Возвращает список голосовых сообщений между двумя пользователями
//	@Tags			voice
//	@Accept			json
//	@Produce		json
//	@Param			sender_id	query		string	true	"ID отправителя"
//	@Param			receiver_id	query		string	true	"ID получателя"
//	@Success		200			{array}		config.VoiceMessage
//	@Failure		400			{object}	config.ErrorResponse
//	@Failure		500			{object}	config.ErrorResponse
//	@Router			/messages/voice [get]
func GetVoiceMessages(c *gin.Context) {
    senderID := c.Query("sender_id")
    receiverID := c.Query("receiver_id")

    var messages []config.VoiceMessage

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
