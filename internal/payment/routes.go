package payment

import "github.com/gin-gonic/gin"

// RegisterRoutes sets up authenticated payment endpoints.
func RegisterRoutes(
	rg *gin.RouterGroup,
	h *Handler,
	authRequired gin.HandlerFunc,
) {
	// Customer endpoint: requires JWT.
	rg.POST(
		"/bookings/:id/pay",
		authRequired,
		h.Create,
	)

	// Midtrans webhook: uses signature verification.
	rg.POST(
		"/payments/webhook",
		h.Webhook,
	)
}
