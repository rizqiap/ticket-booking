package event

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEventNotFound = errors.New("event not found")

// Repository handles event database queries.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a repository with the database pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

// Create saves a new event with the default draft status.
func (r *Repository) Create(
	ctx context.Context,
	title string,
	description string,
	location string,
	startsAt time.Time,
) (*Event, error) {
	var event Event

	err := r.db.QueryRow(ctx, `
        INSERT INTO events (
            title, description, location, starts_at
        )
        VALUES ($1, $2, $3, $4)
        RETURNING
            id, title, description, location,
            starts_at, status, created_at, updated_at
    `, title, description, location, startsAt).Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.Location,
		&event.StartsAt,
		&event.Status,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	return &event, nil
}

// GetByID loads a published event by ID.
func (r *Repository) GetByID(
	ctx context.Context,
	id int64,
) (*Event, error) {
	var event Event

	err := r.db.QueryRow(ctx, `
        SELECT
            id, title, COALESCE(description, ''),
            location, starts_at, status,
            created_at, updated_at
        FROM events
        WHERE id = $1 AND status = 'published'
    `, id).Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.Location,
		&event.StartsAt,
		&event.Status,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}

		return nil, fmt.Errorf("get event by id: %w", err)
	}

	return &event, nil
}

// List loads published events ordered by start time and ID.
func (r *Repository) List(
	ctx context.Context,
	limit, offset int,
) ([]Event, error) {

	rows, err := r.db.Query(ctx, `
        SELECT
            id, title, COALESCE(description, ''),
            location, starts_at, status,
            created_at, updated_at
        FROM events
        WHERE status = 'published'
        ORDER BY starts_at ASC, id ASC
        LIMIT $1 OFFSET $2
    `, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	// Return an empty JSON array when no events match.
	events := make([]Event, 0)

	for rows.Next() {
		var event Event

		err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Description,
			&event.Location,
			&event.StartsAt,
			&event.Status,
			&event.CreatedAt,
			&event.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		events = append(events, event)
	}

	// Check for errors that occurred while reading rows.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}
