package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository stores payments in PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a repository with the database pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Reserve creates a pending payment only once per booking.
// created=true means this request owns Snap creation.
func (r *Repository) Reserve(
	ctx context.Context,
	bookingID, userID int64,
) (*Payment, bool, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin payment tx: %w", err)
	}

	// Roll back unfinished work even if the request context is cancelled.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	var amount int64
	var status string
	var createdAt, expiresAt time.Time

	// Lock booking while validating and reserving payment.
	err = tx.QueryRow(ctx, `
        SELECT total_price, status, created_at, expires_at
        FROM bookings
        WHERE id = $1 AND user_id = $2
        FOR UPDATE
    `, bookingID, userID).Scan(
		&amount,
		&status,
		&createdAt,
		&expiresAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrBookingNotFound
	}
	if err != nil {
		return nil, false, fmt.Errorf("find booking: %w", err)
	}

	// Only pending, unexpired bookings with a positive amount can be paid.
	if status != "pending" ||
		!time.Now().Before(expiresAt) ||
		amount <= 0 {
		return nil, false, ErrBookingNotPayable
	}

	orderID := fmt.Sprintf("BOOKING-%d", bookingID)

	p := &Payment{
		BookingID:        bookingID,
		Amount:           amount,
		Status:           "pending",
		MidtransOrderID:  orderID,
		BookingCreatedAt: createdAt,
	}

	// Unique booking_id prevents duplicate payment rows.
	err = tx.QueryRow(ctx, `
        INSERT INTO payments (
            booking_id, amount, status,
            midtrans_order_id, created_at, updated_at
        )
        VALUES ($1, $2, 'pending', $3, NOW(), NOW())
        ON CONFLICT (booking_id) DO NOTHING
        RETURNING id
    `, bookingID, amount, orderID).Scan(&p.ID)

	created := true

	if errors.Is(err, pgx.ErrNoRows) {
		// Reuse the payment reserved by an earlier request.
		created = false

		err = tx.QueryRow(ctx, `
            SELECT
                id, booking_id, amount, status,
                midtrans_order_id,
                COALESCE(snap_token, ''),
                COALESCE(redirect_url, '')
            FROM payments
            WHERE booking_id = $1
        `, bookingID).Scan(
			&p.ID,
			&p.BookingID,
			&p.Amount,
			&p.Status,
			&p.MidtransOrderID,
			&p.SnapToken,
			&p.RedirectURL,
		)
	}

	if err != nil {
		return nil, false, fmt.Errorf("reserve payment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit payment: %w", err)
	}

	return p, created, nil
}

// SaveSnap stores Snap details only while the pending payment has no URL.
func (r *Repository) SaveSnap(
	ctx context.Context,
	paymentID int64,
	token, redirectURL string,
) error {

	tag, err := r.db.Exec(ctx, `
        UPDATE payments
        SET
            snap_token = $2,
            redirect_url = $3,
            updated_at = NOW()
        WHERE id = $1
          AND status = 'pending'
          AND redirect_url IS NULL
    `, paymentID, token, redirectURL)

	if err != nil {
		return fmt.Errorf("save snap: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return ErrPaymentState
	}

	return nil
}
