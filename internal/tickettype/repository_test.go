package tickettype

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListByEventIDDatabase(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL")
	}
	// One connection keeps all queries on the session with temporary tables.
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, migration := range []string{"000002_create_events_table.up.sql", "000003_create_ticket_types_table.up.sql"} {
		sql, err := os.ReadFile("../../migrations/" + migration)
		if err != nil {
			t.Fatal(err)
		}
		// Temporary tables shadow real tables; fixtures never modify application data.
		if _, err := pool.Exec(ctx, strings.Replace(string(sql), "CREATE TABLE", "CREATE TEMP TABLE", 1)); err != nil {
			t.Fatal(err)
		}
	}
	_, err = pool.Exec(ctx, `
        INSERT INTO events (id, title, location, starts_at, status) VALUES
            (7, 'Concert', 'Jakarta', NOW() + INTERVAL '1 day', 'published'),
            (8, 'Draft', 'Jakarta', NOW() + INTERVAL '1 day', 'draft'),
            (9, 'Empty', 'Jakarta', NOW() + INTERVAL '1 day', 'published');
        INSERT INTO ticket_types (id, event_id, name, price, total_stock, available_stock) VALUES
            (12, 7, 'VIP', 150000, 100, 80),
            (10, 7, 'Regular', 50000, 50, 0),
            (13, 8, 'Private', 100000, 20, 20);
    `)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	tickets, err := repo.ListByEventID(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 2 || tickets[0].ID != 10 || tickets[1].ID != 12 {
		t.Fatalf("wrong list or ordering: %+v", tickets)
	}
	if tickets[0].AvailableStock != 0 || tickets[1].EventID != 7 || tickets[1].Name != "VIP" || tickets[1].Price != 150000 || tickets[1].TotalStock != 100 || tickets[1].AvailableStock != 80 || tickets[1].CreatedAt.IsZero() || tickets[1].UpdatedAt.IsZero() {
		t.Fatalf("incorrectly scanned data: %+v", tickets)
	}
	tickets, err = repo.ListByEventID(ctx, 9)
	if err != nil || tickets == nil || len(tickets) != 0 {
		t.Fatalf("want non-nil empty list; got %+v, %v", tickets, err)
	}
	for _, id := range []int64{8, 999} {
		tickets, err = repo.ListByEventID(ctx, id)
		if tickets != nil || !errors.Is(err, ErrEventNotFound) {
			t.Fatalf("event %d: want not found; got %+v, %v", id, tickets, err)
		}
	}
}
