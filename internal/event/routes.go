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

	// Creating events requires a valid admin token.
	events.POST(
		"",
		authRequired,
		adminOnly,
		h.Create,
	)
}
