package booking

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrTicketUnavailable = errors.New("ticket unavailable")
	ErrDuplicateBooking  = errors.New("idempotency key used for different request")

	errBookingNotFound = errors.New("booking not found")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByUserID(
	ctx context.Context,
	userID int64,
	limit, offset int,
) ([]Booking, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, ticket_type_id, quantity, total_price, status,
		       idempotency_key, expires_at, created_at, updated_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list booking history: %w", err)
	}
	defer rows.Close()

	bookings := make([]Booking, 0)
	for rows.Next() {
		var b Booking
		if err := rows.Scan(
			&b.ID,
			&b.UserID,
			&b.TicketTypeID,
			&b.Quantity,
			&b.TotalPrice,
			&b.Status,
			&b.IdempotencyKey,
			&b.ExpiresAt,
			&b.CreatedAt,
			&b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan booking history: %w", err)
		}
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate booking history: %w", err)
	}
	return bookings, nil
}

// Public method called by Service
func (r *Repository) Create(
	ctx context.Context,
	userID int64,
	input CreateInput,
	key string,
) (*Booking, error) {

	// 1. Check existing booking
	existing, err := r.findByIdempotencyKey(
		ctx, userID, key,
	)

	if err == nil {
		return matchBooking(existing, input)
	}

	if !errors.Is(err, errBookingNotFound) {
		return nil, err
	}

	// 2. Create booking inside transaction
	created, err := r.createOnce(
		ctx, userID, input, key,
	)

	if err == nil {
		return created, nil
	}

	// 3. Recheck in case of concurrent retry
	// or uncertain transaction result
	existing, lookupErr := r.findByIdempotencyKey(
		ctx, userID, key,
	)

	if lookupErr == nil {
		return matchBooking(existing, input)
	}

	if !errors.Is(lookupErr, errBookingNotFound) {
		return nil, fmt.Errorf(
			"create booking: %v; lookup failed: %w",
			err, lookupErr,
		)
	}

	return nil, err
}

// Find previous booking using user and key
func (r *Repository) findByIdempotencyKey(
	ctx context.Context,
	userID int64,
	key string,
) (*Booking, error) {
	var b Booking

	err := r.db.QueryRow(ctx, `
        SELECT
            id, user_id, ticket_type_id,
            quantity, total_price, status,
            idempotency_key, expires_at,
            created_at, updated_at
        FROM bookings
        WHERE user_id = $1
          AND idempotency_key = $2
    `, userID, key).Scan(
		&b.ID,
		&b.UserID,
		&b.TicketTypeID,
		&b.Quantity,
		&b.TotalPrice,
		&b.Status,
		&b.IdempotencyKey,
		&b.ExpiresAt,
		&b.CreatedAt,
		&b.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errBookingNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"find booking by key: %w", err,
		)
	}

	return &b, nil
}

// Compare existing booking with new request
func matchBooking(
	existing *Booking,
	input CreateInput,
) (*Booking, error) {

	if existing.TicketTypeID != input.TicketTypeID ||
		existing.Quantity != input.Quantity {
		return nil, ErrDuplicateBooking
	}

	return existing, nil
}

// Perform stock reservation and booking INSERT
func (r *Repository) createOnce(
	ctx context.Context,
	userID int64,
	input CreateInput,
	key string,
) (*Booking, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	// Rollback if transaction has not committed
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// 1. Atomic stock reservation
	var price int64

	err = tx.QueryRow(ctx, `
        UPDATE ticket_types AS tt
        SET available_stock = tt.available_stock - $2, updated_at = NOW()
        FROM events AS e
        WHERE tt.id = $1
          AND tt.event_id = e.id
          AND e.status = 'published'
          AND e.starts_at > clock_timestamp()
          AND tt.available_stock >= $2
        RETURNING tt.price
    `, input.TicketTypeID, input.Quantity).Scan(&price)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTicketUnavailable
	}

	if err != nil {
		return nil, fmt.Errorf("reserve stock: %w", err)
	}

	// 2. Calculate total price
	if price > math.MaxInt64/int64(input.Quantity) {
		return nil, ErrInvalidBooking
	}
	totalPrice := price * int64(input.Quantity)

	// 3. Insert booking
	var b Booking

	err = tx.QueryRow(ctx, `
        INSERT INTO bookings (
            user_id,
            ticket_type_id,
            quantity,
            total_price,
            status,
            idempotency_key,
            expires_at
        )
        VALUES (
            $1, $2, $3, $4,
            'pending', $5,
            clock_timestamp() + INTERVAL '10 minutes'
        )
        ON CONFLICT (user_id, idempotency_key)
        DO NOTHING
        RETURNING
            id, user_id, ticket_type_id,
            quantity, total_price, status,
            idempotency_key, expires_at,
            created_at, updated_at
    `,
		userID,
		input.TicketTypeID,
		input.Quantity,
		totalPrice,
		key,
	).Scan(
		&b.ID,
		&b.UserID,
		&b.TicketTypeID,
		&b.Quantity,
		&b.TotalPrice,
		&b.Status,
		&b.IdempotencyKey,
		&b.ExpiresAt,
		&b.CreatedAt,
		&b.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDuplicateBooking
	}

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23503" &&
			pgErr.ConstraintName == "bookings_user_id_fkey" {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("insert booking: %w", err)
	}

	// 4. Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit booking: %w", err)
	}

	return &b, nil
}
