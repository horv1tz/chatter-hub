package config

// SimpleResponse представляет простой ответ с сообщением
type SimpleResponse struct {
    Message string `json:"message" example:"Операция выполнена успешно"`
}

// ErrorResponse представляет ошибку с сообщением об ошибке
type ErrorResponse struct {
    Error string `json:"error" example:"Описание ошибки"`
}
