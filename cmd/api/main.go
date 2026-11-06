package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/SeladaKeju/ticket-booking/docs"
	"github.com/SeladaKeju/ticket-booking/internal/auth"
	"github.com/SeladaKeju/ticket-booking/internal/booking"
	"github.com/SeladaKeju/ticket-booking/internal/config"
	"github.com/SeladaKeju/ticket-booking/internal/database"
	"github.com/SeladaKeju/ticket-booking/internal/event"
	"github.com/SeladaKeju/ticket-booking/internal/tickettype"
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

	// Swagger UI and OpenAPI contract.
	docs.RegisterRoutes(r)

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

	// Event dependency injection
	eventRepo := event.NewRepository(db)
	eventService := event.NewService(eventRepo)
	eventHandler := event.NewHandler(eventService)

	// Ticket Type Dependency Injection
	ticketTypeRepo := tickettype.NewRepository(db)
	ticketTypeService := tickettype.NewService(ticketTypeRepo)
	ticketTypeHandler := tickettype.NewHandler(ticketTypeService)

	bookingRepo := booking.NewRepository(db)
	bookingService := booking.NewService(bookingRepo)
	bookingHandler := booking.NewHandler(bookingService)

	// Register API routes
	api := r.Group("/api/v1")
	auth.RegisterRoutes(api, authHandler, authRequired)
	booking.RegisterRoutes(api, bookingHandler, authRequired)

	// Event routes
	event.RegisterRoutes(
		api,
		eventHandler,
		authRequired,
		auth.RequireRole("admin"),
	)

	// Register routes
	tickettype.RegisterRoutes(
		api,
		ticketTypeHandler,
		authRequired,
		auth.RequireRole("admin"),
	)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.AppPort)

	fmt.Printf("Starting API server on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
