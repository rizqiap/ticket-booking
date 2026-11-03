
package tickettype

import (
    "context"
    "errors"
    "strings"
    "unicode/utf8"
)

var ErrInvalidTicketType = errors.New("invalid ticket type data")

type Service struct {
    repo *Repository
}

func NewService(repo *Repository) *Service {
    return &Service{repo: repo}
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
