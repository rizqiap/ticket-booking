package tickettype

import "time"

type CreateInput struct {
	Name       string `json:"name" binding:"required"`
	Price      int64  `json:"price"`
	TotalStock int32  `json:"total_stock"`
}

type TicketType struct {
	ID             int64     `json:"id"`
	EventID        int64     `json:"event_id"`
	Name           string    `json:"name"`
	Price          int64     `json:"price"`
	TotalStock     int32     `json:"total_stock"`
	AvailableStock int32     `json:"available_stock"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpdateInput struct {
	Name       *string `json:"name"`
	Price      *int64  `json:"price"`
	TotalStock *int32  `json:"total_stock"`
}
