package booking

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	bookings := rg.Group("/bookings")
	bookings.Use(authRequired)
	bookings.POST("", h.Create)
	bookings.GET("", h.List)
}
