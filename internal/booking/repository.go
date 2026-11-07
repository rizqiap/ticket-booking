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
	ErrUserNotFound        = errors.New("user not found")
	ErrTicketUnavailable   = errors.New("ticket unavailable")
	ErrDuplicateBooking    = errors.New("idempotency key used for different request")
	ErrBookingCannotCancel = errors.New("booking cannot be cancelled")

	errBookingNotFound = errors.New("booking not found")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Cancel(
	ctx context.Context,
	userID, id int64,
) (*Booking, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cancellation: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	// Lock stock before the booking, matching Create's lock order to avoid deadlocks.
	var ticketTypeID int64
	err = tx.QueryRow(ctx, `
		SELECT tt.id
		FROM ticket_types tt JOIN bookings b ON b.ticket_type_id = tt.id
		WHERE b.id = $1 AND b.user_id = $2
		FOR UPDATE OF tt
	`, id, userID).Scan(&ticketTypeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock cancellation stock: %w", err)
	}

	var b Booking
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, ticket_type_id, quantity, total_price, status,
		       idempotency_key, expires_at, created_at, updated_at
		FROM bookings
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, id, userID).Scan(
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
		return nil, fmt.Errorf("lock cancellation booking: %w", err)
	}
	// Recheck status under the lock so simultaneous cancellations release stock only once.
	if b.Status == "cancelled" {
		return &b, nil
	}
	if b.Status != "pending" {
		return nil, ErrBookingCannotCancel
	}

	err = tx.QueryRow(ctx, `
		UPDATE bookings SET status = 'cancelled', updated_at = clock_timestamp()
		WHERE id = $1
		RETURNING updated_at
	`, id).Scan(&b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("cancel booking: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ticket_types
		SET available_stock = available_stock + $2, updated_at = clock_timestamp()
		WHERE id = $1
	`, ticketTypeID, b.Quantity); err != nil {
		return nil, fmt.Errorf("restore cancellation stock: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancellation: %w", err)
	}
	b.Status = "cancelled"
	return &b, nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	userID, id int64,
) (*Booking, error) {
	var b Booking
	// Match both ID and owner so another user's booking cannot be returned.
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, ticket_type_id, quantity, total_price, status,
		       idempotency_key, expires_at, created_at, updated_at
		FROM bookings
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(
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
		// Hide whether the booking is missing or belongs to someone else.
		return nil, errBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get booking by ID: %w", err)
	}
	return &b, nil
}

func (r *Repository) ListByUserID(
	ctx context.Context,
	userID int64,
	limit, offset int,
) ([]Booking, error) {
	// Sort newest first; ID breaks ties when bookings share the same timestamp.
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
	// Release the query resources even if scanning a row fails.
	defer rows.Close()

	// An empty slice encodes as [] instead of null when the user has no history.
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
	// Next stops on both end-of-results and read errors; check which one occurred.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate booking history: %w", err)
	}
	return bookings, nil
}

// Create reuses a matching booking on retries, or creates one for an unused key.
func (r *Repository) Create(
	ctx context.Context,
	userID int64,
	input CreateInput,
	key string,
) (*Booking, error) {

	// Check user + key before reserving stock so a retry cannot reserve it twice.
	existing, err := r.findByIdempotencyKey(
		ctx, userID, key,
	)

	if err == nil {
		return matchBooking(existing, input)
	}

	if !errors.Is(err, errBookingNotFound) {
		return nil, err
	}

	// Reserve stock and insert the booking in the same transaction.
	created, err := r.createOnce(
		ctx, userID, input, key,
	)

	if err == nil {
		return created, nil
	}

	// Another request may have committed first, or the commit result may be uncertain.
	// Read again before returning the failure, so a retry can receive the saved booking.
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

// The same key can belong to different users, so every lookup includes the user ID.
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

// A reused key must keep the same ticket type and quantity.
func matchBooking(
	existing *Booking,
	input CreateInput,
) (*Booking, error) {

	if existing.TicketTypeID != input.TicketTypeID ||
		existing.Quantity != input.Quantity {
		return nil, ErrDuplicateBooking
	}

	// Return stored values without recalculating price or extending the payment deadline.
	return existing, nil
}

// createOnce commits the stock reservation and booking together, or rolls both back.
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

	// Cleanup gets its own timeout so it can run even if the request was canceled.
	// Rollback after a successful commit does not undo the committed changes.
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// UPDATE locks the ticket row; concurrent requests wait and check the latest stock.
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
		// No reservation means the ticket/event is unavailable or stock is insufficient.
		return nil, ErrTicketUnavailable
	}

	if err != nil {
		return nil, fmt.Errorf("reserve stock: %w", err)
	}

	// Reject multiplication that would overflow int64 and produce an incorrect total.
	if price > math.MaxInt64/int64(input.Quantity) {
		return nil, ErrInvalidBooking
	}
	totalPrice := price * int64(input.Quantity)

	// The unique user/key constraint prevents two concurrent requests from inserting twice.
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
		// ON CONFLICT inserted nothing; deferred rollback restores the reserved stock.
		return nil, ErrDuplicateBooking
	}

	if err != nil {
		var pgErr *pgconn.PgError
		// A valid token may refer to a user who has since been deleted.
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23503" &&
			pgErr.ConstraintName == "bookings_user_id_fkey" {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("insert booking: %w", err)
	}

	// Persist both changes only after the reservation and insert have succeeded.
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit booking: %w", err)
	}

	return &b, nil
}
