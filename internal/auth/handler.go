package auth

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles auth requests and responses.
type Handler struct {
	service *Service
}

// NewHandler creates a handler with the auth service.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Register handles user registration.
func (h *Handler) Register(c *gin.Context) {
	var input RegisterInput

	// Parse and validate input.
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

	// Map errors to HTTP responses.
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

	// Send the registered user's profile.
	c.JSON(http.StatusCreated, gin.H{
		"message": "user registered successfully",
		"data":    user,
	})
}

// Login verifies credentials and returns an access token.
func (h *Handler) Login(c *gin.Context) {
	var input LoginInput

	// Parse and validate input.
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request data",
		})
		return
	}

	// Verify credentials and create a token.
	token, err := h.service.Login(
		c.Request.Context(),
		input,
	)

	// Map errors to HTTP responses.
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid email or password",
			})
			return
		}

		log.Printf("login failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	// Send the token with its lifetime in seconds.
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   3600,
		},
	})
}

// Me returns the authenticated user's current profile.
func (h *Handler) Me(c *gin.Context) {
	// Read the user ID set by auth middleware.
	userID := c.GetInt64("user_id")

	// Load the latest profile from the database.
	user, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}

		log.Printf("get profile failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": user,
	})
}
