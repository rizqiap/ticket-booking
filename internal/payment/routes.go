package payment

import "github.com/gin-gonic/gin"

// RegisterRoutes sets up authenticated payment endpoints.
func RegisterRoutes(
	rg *gin.RouterGroup,
	h *Handler,
	authRequired gin.HandlerFunc,
) {
	rg.POST(
		"/bookings/:id/pay",
		authRequired,
		h.Create,
	)
}
