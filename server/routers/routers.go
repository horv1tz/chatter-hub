package routers

import (
    "github.com/gin-gonic/gin"
    "chatter-hub-server/routers/text"
    "chatter-hub-server/routers/users"
    "chatter-hub-server/routers/voice"
)

// RegisterRoutes registers unprotected routes
func RegisterRoutes(router *gin.Engine) {
    // Unprotected routes
    router.POST("/users", users.CreateUser)  // Create user
    router.POST("/login", users.LoginUser)   // User login
}

// RegisterProtectedRoutes registers protected routes
func RegisterProtectedRoutes(router *gin.RouterGroup) {
    // Protected routes for users
    userGroup := router.Group("/users")
    {
        userGroup.GET("/:id", users.GetUser)
        userGroup.PUT("/:id", users.UpdateUser)
        userGroup.POST("/:id/deactivate", users.DeactivateUser) // Новый маршрут
        userGroup.POST("/:id/activate", users.ActivateUser)     // Новый маршрут
    }

    // Protected routes for text messages
    textGroup := router.Group("/messages/text")
    {
        textGroup.POST("/", text.SendTextMessage)
        textGroup.GET("/", text.GetTextMessages)
    }

    // Protected routes for voice messages
    voiceGroup := router.Group("/messages/voice")
    {
        voiceGroup.POST("/", voice.SendVoiceMessage)
        voiceGroup.GET("/", voice.GetVoiceMessages)
    }
}
