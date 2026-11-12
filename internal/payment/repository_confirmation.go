package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ConfirmNotification atomically applies the service's mapped local states.
func (r *Repository) ConfirmNotification(
	ctx context.Context,
	notificationID int64,
	paymentStatus PaymentStatus,
	bookingStatus BookingStatus,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf(
			"begin confirmation tx: %w", err,
		)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	// 1. Find the payment and booking linked to this notification.
	var paymentID, bookingID, ticketTypeID int64

	err = tx.QueryRow(ctx, `
        SELECT p.id, b.id, b.ticket_type_id
        FROM payment_notifications n
        JOIN payments p ON p.id = n.payment_id
        JOIN bookings b ON b.id = p.booking_id
        WHERE n.id = $1
    `, notificationID).Scan(
		&paymentID,
		&bookingID,
		&ticketTypeID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidNotification
	}

	if err != nil {
		return fmt.Errorf(
			"lookup confirmation: %w", err,
		)
	}

	// 2. Lock stock before booking, matching cancellation's lock order.
	err = tx.QueryRow(ctx, `
        SELECT id
        FROM ticket_types
        WHERE id = $1
        FOR UPDATE
    `, ticketTypeID).Scan(&ticketTypeID)

	if err != nil {
		return fmt.Errorf(
			"lock confirmation stock: %w", err,
		)
	}

	var currentBooking BookingStatus
	var currentPayment PaymentStatus
	var quantity int

	err = tx.QueryRow(ctx, `
        SELECT status, quantity
        FROM bookings
        WHERE id = $1
        FOR UPDATE
    `, bookingID).Scan(
		&currentBooking,
		&quantity,
	)

	if err != nil {
		return fmt.Errorf(
			"lock confirmation booking: %w", err,
		)
	}

	// 3. Lock the payment and check whether the notification was processed.
	err = tx.QueryRow(ctx, `
        SELECT status
        FROM payments
        WHERE id = $1
        FOR UPDATE
    `, paymentID).Scan(&currentPayment)

	if err != nil {
		return fmt.Errorf(
			"lock confirmation payment: %w", err,
		)
	}

	var processedAt *time.Time

	err = tx.QueryRow(ctx, `
        SELECT processed_at
        FROM payment_notifications
        WHERE id = $1
        FOR UPDATE
    `, notificationID).Scan(&processedAt)

	if err != nil {
		return fmt.Errorf(
			"lock confirmation notification: %w", err,
		)
	}

	if processedAt != nil {
		return tx.Commit(ctx)
	}

	// 4. Update payment without downgrading an earlier success.
	if currentPayment != PaymentSuccess &&
		(currentPayment == PaymentPending || paymentStatus == PaymentSuccess ||
			(currentPayment == PaymentFailed && currentBooking == BookingPending)) {
		_, err = tx.Exec(ctx, `
            UPDATE payments
            SET
                status = $2::text,
                updated_at = clock_timestamp(),
                paid_at = CASE
                    WHEN $2::text = 'success'
                    THEN COALESCE(paid_at, clock_timestamp())
                    ELSE paid_at
                END
            WHERE id = $1
        `, paymentID, paymentStatus)

		if err != nil {
			return fmt.Errorf(
				"confirm payment: %w", err,
			)
		}

		// 5. Update pending bookings; closed bookings have released their stock.
		if currentBooking == BookingPending {
			_, err = tx.Exec(ctx, `
                UPDATE bookings
                SET
                    status = $2,
                    updated_at = clock_timestamp()
                WHERE id = $1
            `, bookingID, bookingStatus)

			if err != nil {
				return fmt.Errorf(
					"confirm booking: %w", err,
				)
			}

			// Pending bookings retain stock, including a denied payment attempt.
			if bookingStatus == BookingCancelled || bookingStatus == BookingExpired {
				_, err = tx.Exec(ctx, `
                    UPDATE ticket_types
                    SET
                        available_stock = available_stock + $2,
                        updated_at = clock_timestamp()
                    WHERE id = $1
                `, ticketTypeID, quantity)

				if err != nil {
					return fmt.Errorf(
						"restore confirmation stock: %w", err,
					)
				}
			}
		}
	}

	// 6. Mark the notification processed in the same transaction.
	_, err = tx.Exec(ctx, `
        UPDATE payment_notifications
        SET processed_at = clock_timestamp()
        WHERE id = $1
    `, notificationID)

	if err != nil {
		return fmt.Errorf(
			"mark confirmation processed: %w", err,
		)
	}

	// 7. Commit all changes together.
	return tx.Commit(ctx)
}
