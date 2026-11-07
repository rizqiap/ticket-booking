package booking

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	bookings := rg.Group("/bookings")
	// Require authentication for every booking route, including history and detail.
	bookings.Use(authRequired)
	bookings.POST("", h.Create)
	bookings.GET("", h.List)
	bookings.GET("/:id", h.GetByID)
	bookings.POST("/:id/cancel", h.Cancel)
}
