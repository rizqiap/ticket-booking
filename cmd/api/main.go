package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/SeladaKeju/ticket-booking/internal/config"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize gin router
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.AppPort)

	fmt.Printf("Starting API server on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
