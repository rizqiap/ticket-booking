package booking

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidBooking = errors.New("invalid booking data")

type bookingRepository interface {
	Create(context.Context, int64, CreateInput, string) (*Booking, error)
}

type Service struct {
	repo bookingRepository
}

func NewService(repo bookingRepository) *Service {
	return &Service{repo: repo}
}

// Create validates input; the repository handles transactions and idempotency.
func (s *Service) Create(
	ctx context.Context,
	userID int64,
	input CreateInput,
	key string,
) (*Booking, error) {
	key = strings.TrimSpace(key)
	if userID <= 0 ||
		input.TicketTypeID <= 0 ||
		input.Quantity < 1 ||
		input.Quantity > 4 ||
		len(key) < 8 ||
		len(key) > 128 {
		return nil, ErrInvalidBooking
	}
	return s.repo.Create(ctx, userID, input, key)
}
