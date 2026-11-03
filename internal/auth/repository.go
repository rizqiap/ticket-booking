package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailExists  = errors.New("email already registered")
	ErrUserNotFound = errors.New("user not found")
)

// Repository handles user database queries.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a repository with the database pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

// Create saves a new customer account.
func (r *Repository) Create(ctx context.Context, name, email, passwordHash string) (*User, error) {
	var user User

	err := r.db.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash, role)
		VALUES ($1, $2, $3, 'customer')
		RETURNING id, name, email, role
	`, name, email, passwordHash).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Role,
	)

	if err != nil {
		// Detect duplicate emails.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "users_email_key" {
			return nil, ErrEmailExists
		}

		return nil, fmt.Errorf("create user: %w", err)
	}

	return &user, nil
}

// FindByEmail loads credentials for login.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*UserCredentials, error) {
	var account UserCredentials

	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, role, password_hash
		FROM users
		WHERE email = $1
	`, email).Scan(
		&account.User.ID,
		&account.User.Name,
		&account.User.Email,
		&account.User.Role,
		&account.PasswordHash,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf("find user by email: %w", err)
	}

	return &account, nil
}

// FindByID loads the user's public profile.
func (r *Repository) FindByID(ctx context.Context, id int64) (*User, error) {
	var user User

	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, role
		FROM users
		WHERE id = $1
	`, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Role,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf("find user by id: %w", err)
	}

	return &user, nil
}
