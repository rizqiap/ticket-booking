package booking

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request data",
		})
		return
	}

	// Use the authenticated user from middleware, never a user ID supplied in the body.
	booking, err := h.service.Create(c.Request.Context(), c.GetInt64("user_id"), c.GetHeader("Idempotency-Key"), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidBooking):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid booking data",
			})
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
		case errors.Is(err, ErrTicketUnavailable):
			c.JSON(http.StatusConflict, gin.H{
				"error": ErrTicketUnavailable.Error(),
			})
		case errors.Is(err, ErrDuplicateBooking):
			c.JSON(http.StatusConflict, gin.H{
				"error": ErrDuplicateBooking.Error(),
			})
		default:
			log.Printf("create booking: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "booking created successfully",
		"data":    booking,
	})
}
