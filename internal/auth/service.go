package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidName        = errors.New("name cannot be empty")
	ErrPasswordTooLong    = errors.New("password exceed 72 bytes")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// Service handles auth business logic.
type Service struct {
	repo      *Repository
	jwtSecret []byte
}

// AccessClaims holds the user's role and standard JWT identity and expiry claims.
type AccessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// NewService creates a service with the user repository.
func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
	}
}

// Register validates and saves a new user.
func (s *Service) Register(
	ctx context.Context,
	input RegisterInput,
) (*User, error) {
	// Normalize name and email.
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))

	if name == "" {
		return nil, ErrInvalidName
	}

	// Bcrypt allows up to 72 bytes.
	if len([]byte(input.Password)) > 72 {
		return nil, ErrPasswordTooLong
	}

	// Hash the password before saving.
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

// Authentication verifies the email and password.
func (s *Service) Authentication(ctx context.Context, input LoginInput) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	account, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		// Use the same error for unknown emails and wrong passwords.
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("find user: %w", err)
	}

	// Compare the password with its stored hash.
	err = bcrypt.CompareHashAndPassword(
		[]byte(account.PasswordHash),
		[]byte(input.Password),
	)

	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("verify password: %w", err)
	}

	return &account.User, nil
}

// Login verifies credentials and returns a signed access token.
func (s *Service) Login(ctx context.Context, input LoginInput) (string, error) {
	user, err := s.Authentication(ctx, input)
	if err != nil {
		return "", err
	}

	return s.signAccessToken(user)
}

// signAccessToken creates an access token valid for one hour.
func (s *Service) signAccessToken(user *User) (string, error) {
	now := time.Now()

	claims := AccessClaims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			Issuer:    "ticket-booking",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	// HS256 accepts the shared JWT_SECRET; ES256 requires an ECDSA private key.
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}

// GetProfile returns the authenticated user's profile.
func (s *Service) GetProfile(ctx context.Context, userID int64) (*User, error) {
	return s.repo.FindByID(ctx, userID)
}
