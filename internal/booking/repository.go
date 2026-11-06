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
	ErrDuplicateBooking  = errors.New("duplicate booking request")
)

const bookingLifetime = 15 * time.Minute

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	userID int64,
	key string,
	input CreateInput,
) (*Booking, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin booking: %w", err)
	}
	// Undo stock changes if the booking fails, even when the request was canceled.
	defer tx.Rollback(context.Background())

	// UPDATE locks the row so concurrent requests cannot oversell the stock.
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
		return nil, fmt.Errorf("reserve booking stock: %w", err)
	}
	// Prevent overflow when multiplying the price by quantity.
	if price > math.MaxInt64/int64(input.Quantity) {
		return nil, ErrInvalidBooking
	}

	var booking Booking
	err = tx.QueryRow(ctx, `
		INSERT INTO bookings (user_id, ticket_type_id, quantity, total_price, idempotency_key, expires_at)
		VALUES ($1, $2, $3, $4, $5, clock_timestamp() + $6 * INTERVAL '1 second')
		RETURNING id, user_id, ticket_type_id, quantity, total_price, status,
		          idempotency_key, expires_at, created_at, updated_at
	`, userID, input.TicketTypeID, input.Quantity, price*int64(input.Quantity), key, bookingLifetime.Seconds()).Scan(
		&booking.ID,
		&booking.UserID,
		&booking.TicketTypeID,
		&booking.Quantity,
		&booking.TotalPrice,
		&booking.Status,
		&booking.IdempotencyKey,
		&booking.ExpiresAt,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == "23505" && pgErr.ConstraintName == "bookings_idempotency_unique":
				return nil, ErrDuplicateBooking
			case pgErr.Code == "23503" && pgErr.ConstraintName == "bookings_user_id_fkey":
				return nil, ErrUserNotFound
			}
		}
		return nil, fmt.Errorf("create booking: %w", err)
	}
	// Save the stock reservation and booking together.
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit booking: %w", err)
	}
	return &booking, nil
}
