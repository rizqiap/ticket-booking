
package tickettype

import (
    "context"
    "errors"
    "fmt"

    "github.com/jackc/pgx/v5/pgconn"
    "github.com/jackc/pgx/v5/pgxpool"
)

var (
    ErrEventNotFound   = errors.New("event not found")
    ErrTicketTypeExists = errors.New("ticket type already exists")
)

type Repository struct {
    db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
    return &Repository{db: db}
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
