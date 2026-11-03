package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/SeladaKeju/ticket-booking/internal/auth"
	"github.com/SeladaKeju/ticket-booking/internal/config"
	"github.com/SeladaKeju/ticket-booking/internal/database"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Connect to PostgreSQL
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	db, err := database.NewPostgres(ctx, cfg.DBUrl)
	cancel()

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("Database connected successfully")

	// Initialize gin router
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Wire auth dependencies.
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, cfg.JWTSecret)
	authHandler := auth.NewHandler(authService)
	authRequired := auth.AuthMiddleware(cfg.JWTSecret)

	// Register API routes
	api := r.Group("/api/v1")
	auth.RegisterRoutes(api, authHandler, authRequired)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.AppPort)

	fmt.Printf("Starting API server on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
