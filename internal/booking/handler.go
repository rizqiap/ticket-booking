package booking

import (
	"errors"
	"log"
	"net/http"
	"strconv"

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
	// Binding checks JSON types and required fields; the service checks business rules.
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request data",
		})
		return
	}

	// Use the user ID set by JWT middleware, not an owner supplied in the request body.
	booking, err := h.service.Create(
		c.Request.Context(),
		c.GetInt64("user_id"),
		input,
		c.GetHeader("Idempotency-Key"),
	)
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
			// Keep internal details in logs, not in the HTTP response.
			log.Printf("create booking: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
		}
		return
	}

	// Both new bookings and successful retries return the same envelope and status 201.
	c.JSON(http.StatusCreated, gin.H{
		"message": "booking created successfully",
		"data":    booking,
	})
}

func (h *Handler) List(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	page, errPage := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, errLimit := strconv.Atoi(c.DefaultQuery("limit", "10"))
	// Bound page and limit so the offset stays safe and responses stay small.
	if errPage != nil || errLimit != nil ||
		page < 1 || page > 100000 || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid pagination parameters",
		})
		return
	}

	// A user_id query parameter must not override the authenticated user's identity.
	bookings, err := h.service.List(c.Request.Context(), userID, page, limit)
	if err != nil {
		log.Printf("list booking history: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": bookings,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetByID(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking id"})
		return
	}

	booking, err := h.service.GetByID(c.Request.Context(), userID, id)
	if err != nil {
		if errors.Is(err, errBookingNotFound) {
			// Use the same response for missing bookings and bookings owned by another user.
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
			return
		}
		log.Printf("get booking detail: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": booking})
}

func (h *Handler) Cancel(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking id"})
		return
	}

	booking, err := h.service.Cancel(c.Request.Context(), userID, id)
	if err != nil {
		switch {
		case errors.Is(err, errBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
		case errors.Is(err, ErrBookingCannotCancel):
			c.JSON(http.StatusConflict, gin.H{"error": ErrBookingCannotCancel.Error()})
		default:
			log.Printf("cancel booking: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "booking cancelled successfully",
		"data":    booking,
	})
}
