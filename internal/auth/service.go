package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidName = errors.New("name cannot be empty")
	ErrPasswordTooLong = errors.New("password exceed 72 bytes")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) Register(
	ctx context.Context,
	input RegisterInput,
) (*User, error) {
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))

	if name == "" {
		return nil, ErrInvalidName
	}

	if len([]byte(input.Password)) > 72 {
		return nil, ErrPasswordTooLong
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(input.Password),
		bcrypt.DefaultCost,
	)

	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	return s.repo.Create(
		ctx,
		name,
		email,
		string(hashedPassword),
	)
}