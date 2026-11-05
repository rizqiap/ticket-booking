package tickettype

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEventNotFound      = errors.New("event not found")
	ErrTicketTypeExists   = errors.New("ticket type already exists")
	ErrTicketTypeNotFound = errors.New("ticket type not found")
	ErrStockTooLow        = errors.New("total stock below allocated tickets")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByEventID(ctx context.Context, eventID int64) ([]TicketType, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM events WHERE id = $1 AND status = 'published'
        )
    `, eventID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check published event: %w", err)
	}
	if !exists {
		return nil, ErrEventNotFound
	}

	rows, err := r.db.Query(ctx, `
        SELECT id, event_id, name, price, total_stock, available_stock,
               created_at, updated_at
        FROM ticket_types
        WHERE event_id = $1
        ORDER BY id ASC
    `, eventID)
	if err != nil {
		return nil, fmt.Errorf("list ticket types: %w", err)
	}
	defer rows.Close()

	tickets := make([]TicketType, 0)
	for rows.Next() {
		var ticket TicketType
		if err := rows.Scan(
			&ticket.ID, &ticket.EventID, &ticket.Name, &ticket.Price,
			&ticket.TotalStock, &ticket.AvailableStock,
			&ticket.CreatedAt, &ticket.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ticket type: %w", err)
		}
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ticket types: %w", err)
	}

	return tickets, nil
}

func (r *Repository) Create(
	ctx context.Context,
	eventID int64,
	input CreateInput,
) (*TicketType, error) {
	var ticket TicketType

	err := r.db.QueryRow(ctx, `
        INSERT INTO ticket_types (
            event_id,
            name,
            price,
            total_stock,
            available_stock
        )
        VALUES ($1, $2, $3, $4, $4)
        RETURNING
            id, event_id, name, price,
            total_stock, available_stock,
            created_at, updated_at
    `,
		eventID,
		input.Name,
		input.Price,
		input.TotalStock,
	).Scan(
		&ticket.ID,
		&ticket.EventID,
		&ticket.Name,
		&ticket.Price,
		&ticket.TotalStock,
		&ticket.AvailableStock,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == "23503" &&
				pgErr.ConstraintName == "fk_ticket_types_event":
				return nil, ErrEventNotFound

			case pgErr.Code == "23505" &&
				pgErr.ConstraintName == "ticket_types_event_name_unique":
				return nil, ErrTicketTypeExists
			}
		}

		return nil, fmt.Errorf("create ticket type: %w", err)
	}

	return &ticket, nil
}

func (r *Repository) Update(
	ctx context.Context,
	id int64,
	input UpdateInput,
) (*TicketType, error) {
	var ticket TicketType

	err := r.db.QueryRow(ctx, `
        UPDATE ticket_types
        SET
            name = COALESCE($2, name),
            price = COALESCE($3, price),
            total_stock = COALESCE($4, total_stock),
            available_stock = available_stock +
                (COALESCE($4, total_stock) - total_stock),
            updated_at = NOW()
        WHERE id = $1
          AND available_stock +
              (COALESCE($4, total_stock) - total_stock) >= 0
        RETURNING
            id, event_id, name, price,
            total_stock, available_stock,
            created_at, updated_at
    `, id, input.Name, input.Price, input.TotalStock).Scan(
		&ticket.ID,
		&ticket.EventID,
		&ticket.Name,
		&ticket.Price,
		&ticket.TotalStock,
		&ticket.AvailableStock,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool

		checkErr := r.db.QueryRow(
			ctx,
			"SELECT EXISTS (SELECT 1 FROM ticket_types WHERE id = $1)",
			id,
		).Scan(&exists)

		if checkErr != nil {
			return nil, checkErr
		}

		if !exists {
			return nil, ErrTicketTypeNotFound
		}

		return nil, ErrStockTooLow
	}

	if err != nil {
		return nil, err
	}

	return &ticket, nil
}
