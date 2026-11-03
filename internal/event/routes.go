package event

import "github.com/gin-gonic/gin"

// RegisterRoutes sets up public and admin event endpoints.
func RegisterRoutes(
	rg *gin.RouterGroup,
	h *Handler,
	authRequired gin.HandlerFunc,
	adminOnly gin.HandlerFunc,
) {
	events := rg.Group("/events")

	// Public routes
	events.GET("", h.List)
	events.GET("/:id", h.GetByID)

	// Admin routes
	admin := events.Group("")
	admin.Use(authRequired, adminOnly)

	admin.POST("", h.Create)
	admin.PATCH("/:id", h.Update)
	admin.PATCH("/:id/publish", h.Publish)
}
