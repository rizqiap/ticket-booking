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
	"github.com/SeladaKeju/ticket-booking/internal/payment"
	"github.com/SeladaKeju/ticket-booking/internal/tickettype"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	cfg, err = config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the Midtrans gateway.
	midtransGateway, err := payment.NewMidtransGateway(
		cfg.MidtransServerKey,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to PostgreSQL.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	db, err := database.NewPostgres(ctx, cfg.DBUrl)
	cancel()

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("Database connected successfully")

	// Initialize the Gin router.
	r := gin.Default()

	// Swagger UI and OpenAPI contract.
	docs.RegisterRoutes(r)

	// Register the health check endpoint.
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

	// Wire event dependencies.
	eventRepo := event.NewRepository(db)
	eventService := event.NewService(eventRepo)
	eventHandler := event.NewHandler(eventService)

	// Wire ticket type dependencies.
	ticketTypeRepo := tickettype.NewRepository(db)
	ticketTypeService := tickettype.NewService(ticketTypeRepo)
	ticketTypeHandler := tickettype.NewHandler(ticketTypeService)

	// Wire booking dependencies.
	bookingRepo := booking.NewRepository(db)
	bookingService := booking.NewService(bookingRepo)
	bookingHandler := booking.NewHandler(bookingService)

	// Wire payment dependencies.
	paymentRepo := payment.NewRepository(db)
	paymentService := payment.NewService(paymentRepo, midtransGateway)
	paymentHandler := payment.NewHandler(paymentService)

	// Register API routes under /api/v1.
	api := r.Group("/api/v1")
	// Register auth routes.
	auth.RegisterRoutes(api, authHandler, authRequired)
	// Register authenticated booking routes.
	booking.RegisterRoutes(api, bookingHandler, authRequired)

	// Register public and admin event routes.
	event.RegisterRoutes(
		api,
		eventHandler,
		authRequired,
		auth.RequireRole("admin"),
	)

	// Register public and admin ticket type routes.
	tickettype.RegisterRoutes(
		api,
		ticketTypeHandler,
		authRequired,
		auth.RequireRole("admin"),
	)

	// Register authenticated payment routes.
	payment.RegisterRoutes(
		api,
		paymentHandler,
		authRequired,
	)

	// Start the HTTP server.
	addr := fmt.Sprintf(":%d", cfg.AppPort)

	fmt.Printf("Starting API server on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
