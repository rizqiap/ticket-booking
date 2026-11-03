
package tickettype

import "github.com/gin-gonic/gin"

func RegisterRoutes(
    rg *gin.RouterGroup,
    h *Handler,
    authRequired gin.HandlerFunc,
    adminOnly gin.HandlerFunc,
) {
    events := rg.Group("/events")

    admin := events.Group("")
    admin.Use(authRequired, adminOnly)

    admin.POST("/:id/ticket-types", h.Create)
}
