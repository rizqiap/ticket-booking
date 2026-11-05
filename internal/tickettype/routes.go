package tickettype

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	rg *gin.RouterGroup,
	h *Handler,
	authRequired gin.HandlerFunc,
	adminOnly gin.HandlerFunc,
) {
	events := rg.Group("/events")
	events.GET("/:id/ticket-types", h.ListByEventID)

	admin := events.Group("")
	admin.Use(authRequired, adminOnly)

	admin.POST("/:id/ticket-types", h.Create)

	ticketTypes := rg.Group("/ticket-types")
	ticketTypes.Use(authRequired, adminOnly)
	ticketTypes.PATCH("/:id", h.Update)
}
