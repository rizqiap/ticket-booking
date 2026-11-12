package payment

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// PaymentService defines the payment operation used by the handler.
type PaymentService interface {
	Create(
		ctx context.Context,
		bookingID, userID int64,
	) (*Payment, bool, error)

	HandleNotification(
		ctx context.Context,
		n Notification,
	) (bool, error)
}

// Handler handles payment requests and responses.
type Handler struct {
	service PaymentService
}

// NewHandler creates a handler with the payment service.
func NewHandler(service PaymentService) *Handler {
	return &Handler{service: service}
}

// Create returns a new or existing payment URL for the owner's booking.
func (h *Handler) Create(c *gin.Context) {
	// Read the user ID set by auth middleware.
	userID := c.GetInt64("user_id")

	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	// Parse and validate the booking ID.
	bookingID, err := strconv.ParseInt(
		c.Param("id"), 10, 64,
	)

	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid booking id",
		})
		return
	}

	// Prepare payment for the authenticated user's booking.
	p, created, err := h.service.Create(
		c.Request.Context(),
		bookingID,
		userID,
	)

	// Map errors to HTTP responses.
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidPayment):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

		case errors.Is(err, ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})

		case errors.Is(err, ErrBookingNotPayable):
			c.JSON(http.StatusConflict, gin.H{
				"error": "booking cannot be paid",
			})

		case errors.Is(err, ErrPaymentPreparing):
			c.JSON(http.StatusConflict, gin.H{
				"error": "payment is being prepared",
			})

		case errors.Is(err, ErrProviderUnavailable):
			log.Printf("midtrans create: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{
				"error": "payment provider unavailable",
			})

		default:
			log.Printf("create payment: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
		}
		return
	}

	// Return 201 for a new payment and 200 when reusing one.
	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated
	}

	c.JSON(statusCode, gin.H{
		"data": gin.H{
			"booking_id":  p.BookingID,
			"order_id":    p.MidtransOrderID,
			"amount":      p.Amount,
			"status":      p.Status,
			"payment_url": p.RedirectURL,
		},
	})
}

func (h *Handler) Webhook(c *gin.Context) {
	var n Notification

	// Limit request body to 64 KB.
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		64<<10,
	)

	// 1. Parse JSON notification.
	if err := c.ShouldBindJSON(&n); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid notification body",
		})
		return
	}

	// 2. Validate, save, and confirm payment via Service.
	inserted, err := h.service.HandleNotification(
		c.Request.Context(),
		n,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidNotification):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid notification",
			})

		case errors.Is(err, ErrInvalidSignature):
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid signature",
			})

		case errors.Is(err, ErrPaymentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "payment not found",
			})

		case errors.Is(err, ErrAmountMismatch):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "payment amount mismatch",
			})

		default:
			log.Printf("payment webhook error: %v", err)

			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "unable to process notification",
			})
		}
		return
	}

	// 3. Acknowledge after database commit.
	c.JSON(http.StatusOK, gin.H{
		"status":    "accepted",
		"duplicate": !inserted,
	})
}
