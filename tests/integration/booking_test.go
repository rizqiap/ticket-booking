//go:build integration

package integration_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SeladaKeju/ticket-booking/internal/booking"
	"github.com/SeladaKeju/ticket-booking/internal/payment"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBookingReplayPreservesOriginalData(t *testing.T) {
	ctx, db, service := newBookingTest(t)
	input := booking.CreateInput{TicketTypeID: 1, Quantity: 2}
	original, err := service.Create(ctx, 1, input, "booking-replay")
	if err != nil {
		t.Fatal(err)
	}
	// A retry must work even after the event closes and the ticket price changes.
	if _, err := db.Exec(ctx, `
		UPDATE events SET status = 'draft' WHERE id = 1;
		UPDATE ticket_types SET price = 999999 WHERE id = 1;
		UPDATE bookings SET status = 'expired';
	`); err != nil {
		t.Fatal(err)
	}
	want := *original
	want.Status = "expired"
	replay, err := service.Create(ctx, 1, input, "booking-replay")
	if err != nil || replay == nil || *replay != want {
		t.Fatalf("replay=%+v err=%v; want %+v", replay, err, want)
	}
	_, err = service.Create(
		ctx, 1, booking.CreateInput{TicketTypeID: 1, Quantity: 3}, "booking-replay",
	)
	if !errors.Is(err, booking.ErrDuplicateBooking) {
		t.Fatalf("different payload returned %v; want conflict", err)
	}
	var stock, count int
	if err := db.QueryRow(ctx, `
		SELECT available_stock, (SELECT COUNT(*) FROM bookings)
		FROM ticket_types WHERE id = 1
	`).Scan(&stock, &count); err != nil {
		t.Fatal(err)
	}
	if stock != 3 || count != 1 {
		t.Fatalf("retry changed stock=%d bookings=%d", stock, count)
	}
}

func TestCreateBookingFailureRollsBackStock(t *testing.T) {
	for _, tt := range []struct {
		name     string
		userID   int64
		price    int64
		quantity int32
		wantErr  error
	}{
		{
			name:     "price overflow",
			userID:   1,
			price:    math.MaxInt64/2 + 1,
			quantity: 4,
			wantErr:  booking.ErrInvalidBooking,
		},
		{
			name:     "user no longer exists",
			userID:   999,
			price:    150000,
			quantity: 1,
			wantErr:  booking.ErrUserNotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, service := newBookingTest(t)
			if _, err := db.Exec(ctx, `
				UPDATE ticket_types SET price = $1 WHERE id = 1
			`, tt.price); err != nil {
				t.Fatal(err)
			}

			b, err := service.Create(
				ctx, tt.userID,
				booking.CreateInput{TicketTypeID: 1, Quantity: tt.quantity},
				"booking-rollback",
			)
			if !errors.Is(err, tt.wantErr) || b != nil {
				t.Fatalf("booking=%+v err=%v; want %v", b, err, tt.wantErr)
			}
			var stock, count int
			if err := db.QueryRow(ctx, `
				SELECT available_stock, (SELECT COUNT(*) FROM bookings)
				FROM ticket_types WHERE id = 1
			`).Scan(&stock, &count); err != nil {
				t.Fatal(err)
			}
			if stock != 5 || count != 0 {
				t.Fatalf("failed booking left stock=%d bookings=%d", stock, count)
			}
		})
	}
}

func TestBookingHistoryDatabase(t *testing.T) {
	ctx, db, service := newBookingTest(t)
	if _, err := db.Exec(ctx, `
		INSERT INTO bookings (
			id, user_id, ticket_type_id, quantity, total_price, status,
			idempotency_key, expires_at, created_at
		) VALUES
			(10, 1, 1, 1, 150000, 'pending', 'history-10', NOW(), NOW() - INTERVAL '2 days'),
			(11, 1, 1, 1, 150000, 'confirmed', 'history-11', NOW(), NOW() - INTERVAL '1 day'),
			(12, 1, 1, 1, 150000, 'expired', 'history-12', NOW(), NOW() - INTERVAL '1 day'),
			(13, 2, 1, 1, 150000, 'pending', 'history-13', NOW(), NOW());
	`); err != nil {
		t.Fatal(err)
	}

	statuses := map[int64]string{
		10: "pending", 11: "confirmed", 12: "expired", 13: "pending",
	}
	for _, tt := range []struct {
		name   string
		userID int64
		page   int
		wantID []int64
	}{
		{name: "newest first with ID tie-break", userID: 1, page: 1, wantID: []int64{12, 11}},
		{name: "next page", userID: 1, page: 2, wantID: []int64{10}},
		{name: "other user sees own history", userID: 2, page: 1, wantID: []int64{13}},
		{name: "user without history", userID: 3, page: 1},
		{name: "page beyond history", userID: 1, page: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bookings, err := service.List(ctx, tt.userID, tt.page, 2)
			if err != nil || bookings == nil || len(bookings) != len(tt.wantID) {
				t.Fatalf("history=%+v err=%v; want IDs %v", bookings, err, tt.wantID)
			}
			for n, b := range bookings {
				if b.ID != tt.wantID[n] || b.UserID != tt.userID {
					t.Fatalf("history row %d: booking=%d user=%d; want booking=%d user=%d",
						n, b.ID, b.UserID, tt.wantID[n], tt.userID)
				}
				if b.Status != statuses[b.ID] || b.TicketTypeID != 1 ||
					b.Quantity != 1 || b.TotalPrice != 150000 ||
					b.IdempotencyKey != fmt.Sprintf("history-%d", b.ID) {
					t.Fatalf("incorrect booking data: %+v", b)
				}
				if b.CreatedAt.IsZero() || b.UpdatedAt.IsZero() || b.ExpiresAt.IsZero() {
					t.Fatalf("missing booking timestamps: %+v", b)
				}
			}
		})
	}
}

func TestBookingDetailDatabase(t *testing.T) {
	ctx, db, service := newBookingTest(t)
	want, err := service.Create(
		ctx, 1, booking.CreateInput{TicketTypeID: 1, Quantity: 2}, "booking-detail",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		userID   int64
		id       int64
		notFound bool
	}{
		{name: "owner receives full booking", userID: 1, id: want.ID},
		{name: "another user cannot read it", userID: 2, id: want.ID, notFound: true},
		{name: "user without history cannot read it", userID: 3, id: want.ID, notFound: true},
		{name: "missing booking", userID: 1, id: 999, notFound: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetByID(ctx, tt.userID, tt.id)
			if tt.notFound {
				if got != nil || err == nil || err.Error() != "booking not found" {
					t.Fatalf("detail=%+v err=%v; want booking not found", got, err)
				}
				return
			}
			if err != nil || got == nil || *got != *want {
				t.Fatalf("detail=%+v err=%v; want %+v", got, err, want)
			}
		})
	}

	// Completed or expired bookings remain visible in the owner's detail view.
	if _, err := db.Exec(ctx, `
		UPDATE bookings SET status = 'expired' WHERE id = $1
	`, want.ID); err != nil {
		t.Fatal(err)
	}
	want.Status = "expired"
	got, err := service.GetByID(ctx, 1, want.ID)
	if err != nil || got == nil || *got != *want {
		t.Fatalf("expired detail=%+v err=%v; want %+v", got, err, want)
	}
}

func TestCancelBookingDatabase(t *testing.T) {
	for _, tt := range []struct {
		name        string
		status      string
		userID      int64
		missing     bool
		retry       bool
		restoreFail bool
		wantError   string
		wantStock   int
	}{
		{name: "pending", status: "pending", userID: 1, wantStock: 5},
		{name: "retry", status: "pending", userID: 1, retry: true, wantStock: 5},
		{
			name: "confirmed", status: "confirmed", userID: 1,
			wantError: "booking cannot be cancelled", wantStock: 3,
		},
		{
			name: "expired", status: "expired", userID: 1,
			wantError: "booking cannot be cancelled", wantStock: 3,
		},
		{
			name: "other owner", status: "pending", userID: 2,
			wantError: "booking not found", wantStock: 3,
		},
		{
			name: "missing booking", status: "pending", userID: 1, missing: true,
			wantError: "booking not found", wantStock: 3,
		},
		{
			name: "stock failure rolls back status", status: "pending", userID: 1,
			restoreFail: true, wantError: "restore cancellation stock:", wantStock: 3,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, service := newBookingTest(t)
			input := booking.CreateInput{TicketTypeID: 1, Quantity: 2}
			original, err := service.Create(ctx, 1, input, "cancel-booking")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `
				UPDATE bookings SET status = $1 WHERE id = $2
			`, tt.status, original.ID); err != nil {
				t.Fatal(err)
			}
			if tt.restoreFail {
				if _, err := db.Exec(ctx, `
					ALTER TABLE ticket_types ADD CHECK (available_stock <= 3)
				`); err != nil {
					t.Fatal(err)
				}
			}
			id := original.ID
			if tt.missing {
				id = 999
			}
			got, err := service.Cancel(ctx, tt.userID, id)
			if tt.wantError != "" {
				if got != nil || err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("cancellation=%+v err=%v; want %q", got, err, tt.wantError)
				}
			} else {
				if err != nil || got == nil || got.Status != "cancelled" || got.ID != original.ID {
					t.Fatalf("cancellation=%+v err=%v", got, err)
				}
				if tt.retry {
					again, err := service.Cancel(ctx, 1, original.ID)
					if err != nil || again == nil || *again != *got {
						t.Fatalf("retry changed cancellation: %+v err=%v", again, err)
					}
				}
				// Retrying create with the original key must not reserve stock again.
				replay, err := service.Create(ctx, 1, input, "cancel-booking")
				if err != nil || replay == nil || *replay != *got {
					t.Fatalf("create replay after cancellation: %+v err=%v", replay, err)
				}
			}
			var stock, count int
			var status string
			if err := db.QueryRow(ctx, `
				SELECT tt.available_stock, b.status, (SELECT COUNT(*) FROM bookings)
				FROM ticket_types tt JOIN bookings b ON b.ticket_type_id = tt.id
				WHERE b.id = $1
			`, original.ID).Scan(&stock, &status, &count); err != nil {
				t.Fatal(err)
			}
			wantStatus := tt.status
			if tt.wantError == "" {
				wantStatus = "cancelled"
			}
			if stock != tt.wantStock || status != wantStatus || count != 1 {
				t.Fatalf("stock=%d status=%s bookings=%d", stock, status, count)
			}
		})
	}
}

func TestCreateBookingConcurrency(t *testing.T) {
	for _, tt := range []struct {
		name        string
		sameKey     bool
		quantity    int32
		stock       int
		wantCreated int
		wantStock   int
		wantError   error
	}{
		{
			name:        "requests compete for limited stock",
			quantity:    1,
			stock:       5,
			wantCreated: 5,
			wantStock:   0,
			wantError:   booking.ErrTicketUnavailable,
		},
		{
			name:        "same user and key create only one booking",
			sameKey:     true,
			quantity:    2,
			stock:       2,
			wantCreated: 1,
			wantStock:   0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, service := newBookingTest(t)
			if _, err := db.Exec(ctx, `
				UPDATE ticket_types SET total_stock = $1, available_stock = $1 WHERE id = 1
			`, tt.stock); err != nil {
				t.Fatal(err)
			}

			const requests = 20
			start := make(chan struct{})
			type result struct {
				booking *booking.Booking
				err     error
			}
			results := make(chan result, requests)
			for n := 1; n <= requests; n++ {
				go func(userID int64) {
					key := fmt.Sprintf("booking-%03d", userID)
					if tt.sameKey {
						userID = 1
						key = "booking-same-key"
					}
					// Release all requests together, without relying on sleeps.
					<-start
					b, err := service.Create(
						ctx,
						userID,
						booking.CreateInput{TicketTypeID: 1, Quantity: tt.quantity},
						key,
					)
					results <- result{b, err}
				}(int64(n))
			}
			close(start)

			successes := 0
			var original *booking.Booking
			for n := 0; n < requests; n++ {
				result := <-results
				if result.err != nil {
					if !errors.Is(result.err, tt.wantError) {
						t.Errorf("unexpected error: %v; want %v", result.err, tt.wantError)
					}
					continue
				}
				successes++
				if result.booking == nil {
					t.Error("successful request returned no booking")
					continue
				}
				remaining := time.Until(result.booking.ExpiresAt)
				if remaining < 9*time.Minute || remaining > 10*time.Minute+time.Second {
					t.Errorf("unexpected payment deadline: %v remaining", remaining)
				}
				if tt.sameKey {
					if original != nil && *result.booking != *original {
						t.Error("retry changed the original booking")
					}
					original = result.booking
				}
			}
			wantSuccesses := tt.wantCreated
			if tt.sameKey {
				wantSuccesses = requests
			}
			if successes != wantSuccesses {
				t.Errorf("successful responses=%d; want %d", successes, wantSuccesses)
			}

			var stock, count, reserved int
			if err := db.QueryRow(ctx, `
				SELECT available_stock FROM ticket_types WHERE id = 1
			`).Scan(&stock); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(ctx, `
				SELECT COUNT(*), COALESCE(SUM(quantity), 0) FROM bookings
			`).Scan(&count, &reserved); err != nil {
				t.Fatal(err)
			}
			if count != tt.wantCreated ||
				stock != tt.wantStock ||
				stock+reserved != tt.stock {
				t.Fatalf(
					"bookings=%d stock=%d reserved=%d; want bookings=%d stock=%d",
					count, stock, reserved, tt.wantCreated, tt.wantStock,
				)
			}
		})
	}
}

func TestConcurrentCancellationDatabase(t *testing.T) {
	ctx, db, service := newBookingTest(t)
	original, err := service.Create(
		ctx, 1, booking.CreateInput{TicketTypeID: 1, Quantity: 2}, "concurrent-cancel",
	)
	if err != nil {
		t.Fatal(err)
	}
	const requests = 20
	start := make(chan struct{})
	results := make(chan error, requests)
	for n := 0; n < requests; n++ {
		go func() {
			<-start
			b, err := service.Cancel(ctx, 1, original.ID)
			if err == nil && (b == nil || b.Status != "cancelled") {
				err = errors.New("cancellation returned incorrect booking")
			}
			results <- err
		}()
	}
	close(start)
	for n := 0; n < requests; n++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	var stock, count int
	if err := db.QueryRow(ctx, `
		SELECT available_stock, (SELECT COUNT(*) FROM bookings WHERE status = 'cancelled')
		FROM ticket_types WHERE id = 1
	`).Scan(&stock, &count); err != nil {
		t.Fatal(err)
	}
	if stock != 5 || count != 1 {
		t.Fatalf("concurrent cancellation left stock=%d cancelled bookings=%d", stock, count)
	}
}

// newBookingTest gives each scenario an isolated database and a real booking service.
func newBookingTest(t *testing.T) (context.Context, *pgxpool.Pool, *booking.Service) {
	t.Helper()
	db := bookingDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx, db, booking.NewService(booking.NewRepository(db))
}

func bookingDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)

	// Each scenario gets its own schema, shared by all its concurrent connections.
	schema := fmt.Sprintf("booking_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, `DROP SCHEMA `+quoted+` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 20
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	for _, name := range []string{
		"000001_create_users_table.up.sql",
		"000002_create_events_table.up.sql",
		"000003_create_ticket_types_table.up.sql",
		"000004_create_bookings_table.up.sql",
		"000007_add_cancelled_booking_status.up.sql",
	} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash)
		SELECT n, 'Customer', n || '@test.local', 'unused'
		FROM generate_series(1, 20) n;
		INSERT INTO events (id, title, location, starts_at, status)
		VALUES (1, 'Concert', 'Jakarta', NOW() + INTERVAL '1 day', 'published');
		INSERT INTO ticket_types (id, event_id, name, price, total_stock, available_stock)
		VALUES (1, 1, 'VIP', 150000, 5, 5);
	`); err != nil {
		t.Fatal(err)
	}
	return pool
}

// Reuse the existing isolated PostgreSQL fixture for payment confirmation tests.
func newPaymentConfirmationTest(t *testing.T) (context.Context, *pgxpool.Pool, *payment.Service, payment.Notification, int64) {
	t.Helper()
	ctx, db, bookings := newBookingTest(t)
	for _, name := range []string{"000005_create_payments_table.up.sql", "000008_add_midtrans_to_payments.up.sql", "000009_create_payment_notifications.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	b, err := bookings.Create(ctx, 1, booking.CreateInput{TicketTypeID: 1, Quantity: 2}, "confirmation-test")
	if err != nil {
		t.Fatal(err)
	}
	repo := payment.NewRepository(db)
	p, _, err := repo.Reserve(ctx, b.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := payment.NewMidtransGateway("test-api-key", "test-signing-key")
	if err != nil {
		t.Fatal(err)
	}
	n := payment.Notification{OrderID: p.MidtransOrderID, TransactionID: "transaction", TransactionStatus: "settlement", StatusCode: "200", GrossAmount: "300000.00", FraudStatus: "accept", Currency: "IDR"}
	digest := sha512.Sum512([]byte(n.OrderID + n.StatusCode + n.GrossAmount + "test-signing-key"))
	n.SignatureKey = hex.EncodeToString(digest[:])
	return ctx, db, payment.NewService(repo, gateway), n, b.ID
}

func assertConfirmationState(t *testing.T, ctx context.Context, db *pgxpool.Pool, wantPayment, wantBooking string, wantStock int) {
	t.Helper()
	var p, b string
	var stock int
	var paid bool
	if err := db.QueryRow(ctx, `SELECT p.status, b.status, tt.available_stock, p.paid_at IS NOT NULL
        FROM payments p JOIN bookings b ON b.id = p.booking_id JOIN ticket_types tt ON tt.id = b.ticket_type_id`).Scan(&p, &b, &stock, &paid); err != nil {
		t.Fatal(err)
	}
	if p != wantPayment || b != wantBooking || stock != wantStock || paid != (wantPayment == "success") {
		t.Fatalf("got payment=%s booking=%s stock=%d paid=%v", p, b, stock, paid)
	}
}

func TestPaymentConfirmationDatabase(t *testing.T) {
	for _, tt := range []struct {
		status, fraud, p, b string
		stock               int
		processed           bool
	}{
		{"settlement", "accept", "success", "confirmed", 3, true},
		{"capture", "accept", "success", "confirmed", 3, true},
		{"capture", "challenge", "pending", "pending", 3, false},
		{"pending", "accept", "pending", "pending", 3, false},
		{"deny", "deny", "failed", "pending", 3, true},
		{"cancel", "", "failed", "cancelled", 5, true},
		{"expire", "", "failed", "expired", 5, true},
		{"failure", "", "failed", "cancelled", 5, true},
		{"refund", "accept", "pending", "pending", 3, false},
	} {
		t.Run(tt.status+tt.fraud, func(t *testing.T) {
			ctx, db, service, n, _ := newPaymentConfirmationTest(t)
			n.TransactionStatus, n.FraudStatus = tt.status, tt.fraud
			if inserted, err := service.HandleNotification(ctx, n); err != nil || !inserted {
				t.Fatalf("inserted=%v err=%v", inserted, err)
			}
			if inserted, err := service.HandleNotification(ctx, n); err != nil || inserted {
				t.Fatalf("duplicate inserted=%v err=%v", inserted, err)
			}
			assertConfirmationState(t, ctx, db, tt.p, tt.b, tt.stock)
			var count int
			var processed bool
			if err := db.QueryRow(ctx, `SELECT COUNT(*), bool_and(processed_at IS NOT NULL) FROM payment_notifications`).Scan(&count, &processed); err != nil {
				t.Fatal(err)
			}
			if count != 1 || processed != tt.processed {
				t.Fatalf("count=%d processed=%v", count, processed)
			}
		})
	}
}

func TestPaymentConfirmationRollbackAndRetry(t *testing.T) {
	for _, stockFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(stockFailure), func(t *testing.T) {
			ctx, db, service, n, _ := newPaymentConfirmationTest(t)
			constraint := `ALTER TABLE bookings ADD CONSTRAINT confirmation_test CHECK (status <> 'confirmed')`
			wantErr, target, p, b, stock := "confirm booking:", "bookings", "success", "confirmed", 3
			if stockFailure {
				n.TransactionStatus = "expire"
				constraint = `ALTER TABLE ticket_types ADD CONSTRAINT confirmation_test CHECK (available_stock <= 3)`
				wantErr, target, p, b, stock = "restore confirmation stock:", "ticket_types", "failed", "expired", 5
			}
			if _, err := db.Exec(ctx, constraint); err != nil {
				t.Fatal(err)
			}
			if _, err := service.HandleNotification(ctx, n); err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("expected %s, got %v", wantErr, err)
			}
			assertConfirmationState(t, ctx, db, "pending", "pending", 3)
			var count int
			if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM payment_notifications WHERE processed_at IS NULL`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatal("failed confirmation must retain the unprocessed notification")
			}
			if _, err := db.Exec(ctx, `ALTER TABLE `+target+` DROP CONSTRAINT confirmation_test`); err != nil {
				t.Fatal(err)
			}
			if inserted, err := service.HandleNotification(ctx, n); err != nil || inserted {
				t.Fatalf("retry inserted=%v err=%v", inserted, err)
			}
			assertConfirmationState(t, ctx, db, p, b, stock)
		})
	}
}

func TestPaymentConfirmationReplayAndLateFailure(t *testing.T) {
	ctx, db, service, n, _ := newPaymentConfirmationTest(t)
	// Receipt saved by 9C must still be confirmable by its existing notification ID.
	id, _, err := payment.NewRepository(db).SaveNotification(ctx, n, 300000)
	if err != nil || id <= 0 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if inserted, err := service.HandleNotification(ctx, n); err != nil || inserted {
		t.Fatalf("replay inserted=%v err=%v", inserted, err)
	}
	var paidAt time.Time
	if err := db.QueryRow(ctx, `SELECT paid_at FROM payments`).Scan(&paidAt); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"pending", "expire", "deny", "settlement"} {
		n.TransactionStatus = status
		if _, err := service.HandleNotification(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	assertConfirmationState(t, ctx, db, "success", "confirmed", 3)
	var finalPaidAt time.Time
	if err := db.QueryRow(ctx, `SELECT paid_at FROM payments`).Scan(&finalPaidAt); err != nil {
		t.Fatal(err)
	}
	if !finalPaidAt.Equal(paidAt) {
		t.Fatal("replay changed paid_at")
	}
}

func TestPaymentConfirmationRacesCancellation(t *testing.T) {
	ctx, db, service, n, bookingID := newPaymentConfirmationTest(t)
	start := make(chan struct{})
	results := make(chan error, 6)
	for i := 0; i < 5; i++ {
		go func() { <-start; _, err := service.HandleNotification(ctx, n); results <- err }()
	}
	go func() {
		<-start
		_, err := booking.NewService(booking.NewRepository(db)).Cancel(ctx, 1, bookingID)
		if errors.Is(err, booking.ErrBookingCannotCancel) {
			err = nil
		}
		results <- err
	}()
	close(start)
	for i := 0; i < 6; i++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	var status string
	if err := db.QueryRow(ctx, `SELECT status FROM bookings WHERE id = $1`, bookingID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	switch status {
	case "confirmed":
		assertConfirmationState(t, ctx, db, "success", "confirmed", 3)
	case "cancelled":
		assertConfirmationState(t, ctx, db, "success", "cancelled", 5)
	default:
		t.Fatalf("unexpected booking status: %s", status)
	}
}

func TestDeniedPaymentRetainsStockUntilFinalOutcome(t *testing.T) {
	for _, tt := range []struct {
		status, payment, booking string
		stock                    int
	}{
		{"settlement", "success", "confirmed", 3},
		{"expire", "failed", "expired", 5},
		{"cancel", "failed", "cancelled", 5},
	} {
		t.Run(tt.status, func(t *testing.T) {
			ctx, db, service, n, _ := newPaymentConfirmationTest(t)
			n.TransactionStatus = "deny"
			if _, err := service.HandleNotification(ctx, n); err != nil {
				t.Fatal(err)
			}
			assertConfirmationState(t, ctx, db, "failed", "pending", 3)
			n.TransactionStatus = tt.status
			if _, err := service.HandleNotification(ctx, n); err != nil {
				t.Fatal(err)
			}
			if _, err := service.HandleNotification(ctx, n); err != nil {
				t.Fatal(err)
			}
			assertConfirmationState(t, ctx, db, tt.payment, tt.booking, tt.stock)
		})
	}
}
