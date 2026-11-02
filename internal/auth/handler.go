package auth

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
	return &Handler{
		service: service,
	}
}

func (h *Handler) Register(c *gin.Context) {
	var input RegisterInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request data",
		})
		return
	}

	user, err := h.service.Register(
		c.Request.Context(),
		input,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrEmailExists):
			c.JSON(http.StatusConflict, gin.H{
				"error": "email already registered",
			})
		case errors.Is(err, ErrInvalidName),
			errors.Is(err, ErrPasswordTooLong):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
		default:
			log.Printf("register failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "user registered seccessfully",
		"data": user,
	})
}
