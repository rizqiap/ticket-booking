package booking

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidBooking = errors.New("invalid booking data")

type bookingRepository interface {
	Create(context.Context, int64, CreateInput, string) (*Booking, error)
	ListByUserID(context.Context, int64, int, int) ([]Booking, error)
	GetByID(context.Context, int64, int64) (*Booking, error)
	Cancel(context.Context, int64, int64) (*Booking, error)
}

type Service struct {
	repo bookingRepository
}

func NewService(repo bookingRepository) *Service {
	return &Service{repo: repo}
}

// Create validates input; the repository handles transactions and idempotent retries.
func (s *Service) Create(
	ctx context.Context,
	userID int64,
	input CreateInput,
	key string,
) (*Booking, error) {
	key = strings.TrimSpace(key)
	// Enforce 1-4 tickets and an 8-128 byte key after trimming surrounding whitespace.
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

func (s *Service) List(
	ctx context.Context,
	userID int64,
	page, limit int,
) ([]Booking, error) {
	// Offset skips earlier pages; limit caps how many rows this page returns.
	offset := (page - 1) * limit
	return s.repo.ListByUserID(ctx, userID, limit, offset)
}

func (s *Service) GetByID(
	ctx context.Context,
	userID, id int64,
) (*Booking, error) {
	return s.repo.GetByID(ctx, userID, id)
}

func (s *Service) Cancel(
	ctx context.Context,
	userID, id int64,
) (*Booking, error) {
	return s.repo.Cancel(ctx, userID, id)
}
