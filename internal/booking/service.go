package booking

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidBooking = errors.New("invalid booking data")

type bookingRepository interface {
	Create(context.Context, int64, string, CreateInput) (*Booking, error)
}

type Service struct {
	repo bookingRepository
}

func NewService(repo bookingRepository) *Service {
	return &Service{repo: repo}
}

// Create validates the request before reserving stock.
func (s *Service) Create(ctx context.Context, userID int64, key string, input CreateInput) (*Booking, error) {
	key = strings.TrimSpace(key)
	// Allow 1-4 tickets and an idempotency key of 8-128 bytes after trimming.
	if userID <= 0 ||
		input.TicketTypeID <= 0 ||
		input.Quantity < 1 ||
		input.Quantity > 4 ||
		len(key) < 8 ||
		len(key) > 128 {
		return nil, ErrInvalidBooking
	}
	return s.repo.Create(
		ctx,
		userID,
		key,
		input,
	)
}
