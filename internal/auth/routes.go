package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes sets up auth endpoints.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	auth := rg.Group("/auth")

	// Public routes.
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)

	// Protected routes require a valid token.
	protected := auth.Group("")
	protected.Use(authRequired)

	protected.GET("/me", h.Me)
}
