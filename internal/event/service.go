package event

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidEvent = errors.New("invalid event data")

// Service handles event business logic.
type Service struct {
	repo *Repository
}

// NewService creates a service with the event repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and saves a new event.
func (s *Service) Create(
	ctx context.Context,
	input CreateEventInput,
) (*Event, error) {

	// Normalize required text fields.
	title := strings.TrimSpace(input.Title)
	location := strings.TrimSpace(input.Location)

	if title == "" || location == "" {
		return nil, ErrInvalidEvent
	}

	// Match database character limits, including Unicode text.
	if utf8.RuneCountInString(title) > 255 ||
		utf8.RuneCountInString(location) > 255 {
		return nil, ErrInvalidEvent
	}

	// Events must start in the future.
	if !input.StartsAt.After(time.Now()) {
		return nil, ErrInvalidEvent
	}

	return s.repo.Create(
		ctx,
		title,
		input.Description,
		location,
		input.StartsAt,
	)
}

// GetByID returns a published event.
func (s *Service) GetByID(
	ctx context.Context,
	id int64,
) (*Event, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns one page of published events.
func (s *Service) List(
	ctx context.Context,
	page, limit int,
) ([]Event, error) {

	// Convert the page number into the number of rows to skip.
	offset := (page - 1) * limit

	return s.repo.List(ctx, limit, offset)
}
