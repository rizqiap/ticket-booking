package event

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler handles event requests and responses.
type Handler struct {
	service *Service
}

// NewHandler creates a handler with the event service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Create handles event creation for admins.
func (h *Handler) Create(c *gin.Context) {
	var input CreateEventInput

	// Parse and validate input.
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request data",
		})
		return
	}

	event, err := h.service.Create(
		c.Request.Context(),
		input,
	)

	if err != nil {
		if errors.Is(err, ErrInvalidEvent) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid event data",
			})
			return
		}

		log.Printf("create event failed: %v", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "event created successfully",
		"data":    event,
	})
}

// GetByID returns a published event by URL ID.
func (h *Handler) GetByID(c *gin.Context) {
	// 1. Read URL parameter
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid event id",
		})
		return
	}

	// 2. Call service
	event, err := h.service.GetByID(
		c.Request.Context(),
		id,
	)

	if err != nil {
		if errors.Is(err, ErrEventNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "event not found",
			})
			return
		}

		log.Printf("get event failed: %v", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	// 3. Send response
	c.JSON(http.StatusOK, gin.H{
		"data": event,
	})
}

// List returns published events with pagination.
func (h *Handler) List(c *gin.Context) {
	// 1. Read query parameters
	page, errPage := strconv.Atoi(
		c.DefaultQuery("page", "1"),
	)

	limit, errLimit := strconv.Atoi(
		c.DefaultQuery("limit", "10"),
	)

	// 2. Validate pagination
	if errPage != nil || errLimit != nil ||
		page < 1 || limit < 1 || limit > 100 ||
		page > 100000 {

		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid pagination parameters",
		})
		return
	}

	// 3. Call service
	events, err := h.service.List(
		c.Request.Context(),
		page,
		limit,
	)

	if err != nil {
		log.Printf("list events failed: %v", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	// 4. Send response
	c.JSON(http.StatusOK, gin.H{
		"data": events,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
		},
	})
}

// Update handles partial event updates for admins.
func (h *Handler) Update(c *gin.Context) {
	// 1. Parse event ID
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid event id",
		})
		return
	}

	// 2. Read JSON request
	var input UpdateEventInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request data",
		})
		return
	}

	// 3. Call service
	event, err := h.service.Update(
		c.Request.Context(),
		id,
		input,
	)

	// 4. Handle errors
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidEvent):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid event data",
			})

		case errors.Is(err, ErrEventNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "event not found",
			})

		default:
			log.Printf("update event failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
		}
		return
	}

	// 5. Send HTTP response
	c.JSON(http.StatusOK, gin.H{
		"message": "event updated successfully",
		"data":    event,
	})
}

// Publish handles publishing draft events for admins.
func (h *Handler) Publish(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)

	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid event id",
		})
		return
	}

	event, err := h.service.Publish(
		c.Request.Context(),
		id,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrEventNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "event not found",
			})

		case errors.Is(err, ErrEventCannotPublish):
			c.JSON(http.StatusConflict, gin.H{
				"error": "event cannot be published",
			})

		default:
			log.Printf("publish event failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "event published successfully",
		"data":    event,
	})
}
