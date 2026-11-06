package booking

import "time"

type CreateInput struct {
	TicketTypeID int64 `json:"ticket_type_id" binding:"required"`
	Quantity     int32 `json:"quantity" binding:"required"`
}

type Booking struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	TicketTypeID   int64     `json:"ticket_type_id"`
	Quantity       int32     `json:"quantity"`
	TotalPrice     int64     `json:"total_price"`
	Status         string    `json:"status"`
	IdempotencyKey string    `json:"idempotency_key"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
