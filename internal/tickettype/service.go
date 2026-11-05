package tickettype

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidTicketType = errors.New("invalid ticket type data")

type ticketTypeRepository interface {
	Create(context.Context, int64, CreateInput) (*TicketType, error)
	Update(context.Context, int64, UpdateInput) (*TicketType, error)
	ListByEventID(context.Context, int64) ([]TicketType, error)
}

type Service struct {
	repo ticketTypeRepository
}

func NewService(repo ticketTypeRepository) *Service {
	return &Service{repo: repo}
}

// ListByEventID returns the ticket types of a published event.
func (s *Service) ListByEventID(ctx context.Context, eventID int64) ([]TicketType, error) {
	return s.repo.ListByEventID(ctx, eventID)
}

func (s *Service) Create(
	ctx context.Context,
	eventID int64,
	input CreateInput,
) (*TicketType, error) {

	// Normalize name
	input.Name = strings.TrimSpace(input.Name)

	// Validate input
	if input.Name == "" ||
		utf8.RuneCountInString(input.Name) > 100 ||
		input.Price <= 0 ||
		input.TotalStock <= 0 {
		return nil, ErrInvalidTicketType
	}

	// Save to database
	return s.repo.Create(ctx, eventID, input)
}

func (s *Service) Update(
	ctx context.Context,
	id int64,
	input UpdateInput,
) (*TicketType, error) {

	if input.Name == nil &&
		input.Price == nil &&
		input.TotalStock == nil {
		return nil, ErrInvalidTicketType
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)

		if name == "" {
			return nil, ErrInvalidTicketType
		}

		input.Name = &name
	}

	if input.Price != nil && *input.Price <= 0 {
		return nil, ErrInvalidTicketType
	}

	if input.TotalStock != nil && *input.TotalStock <= 0 {
		return nil, ErrInvalidTicketType
	}

	return s.repo.Update(ctx, id, input)
}
