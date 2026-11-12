package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) SaveNotification(
	ctx context.Context,
	n Notification,
	grossAmount int64,
) (int64, bool, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, false, fmt.Errorf(
			"begin notification tx: %w", err,
		)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	// 1. Find our payment by Midtrans order ID.
	var paymentID, expectedAmount int64

	err = tx.QueryRow(ctx, `
        SELECT id, amount
        FROM payments
        WHERE midtrans_order_id = $1
        FOR UPDATE
    `, n.OrderID).Scan(
		&paymentID,
		&expectedAmount,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, ErrPaymentNotFound
	}

	if err != nil {
		return 0, false, fmt.Errorf(
			"lookup payment: %w", err,
		)
	}

	// 2. Verify amount against our database.
	if expectedAmount != grossAmount {
		return 0, false, ErrAmountMismatch
	}

	// 3. Store notification once.
	tag, err := tx.Exec(ctx, `
        INSERT INTO payment_notifications (
            payment_id,
            transaction_id,
            transaction_status,
            status_code,
            fraud_status
        )
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT ON CONSTRAINT
            payment_notification_unique
        DO NOTHING
    `,
		paymentID,
		n.TransactionID,
		n.TransactionStatus,
		n.StatusCode,
		n.FraudStatus,
	)

	if err != nil {
		return 0, false, fmt.Errorf(
			"insert notification: %w", err,
		)
	}

	// 4. Read the notification ID, including an existing one on retries.
	var notificationID int64

	err = tx.QueryRow(ctx, `
        SELECT id
        FROM payment_notifications
        WHERE payment_id = $1
          AND transaction_id = $2
          AND transaction_status = $3
          AND status_code = $4
          AND fraud_status = $5
    `,
		paymentID,
		n.TransactionID,
		n.TransactionStatus,
		n.StatusCode,
		n.FraudStatus,
	).Scan(&notificationID)

	if err != nil {
		return 0, false, fmt.Errorf(
			"lookup notification: %w", err,
		)
	}

	// 5. Commit receipt before confirmation so failures can be retried by ID.
	if err := tx.Commit(ctx); err != nil {
		return 0, false, fmt.Errorf(
			"commit notification: %w", err,
		)
	}

	return notificationID, tag.RowsAffected() == 1, nil
}
