
CREATE TABLE ticket_types (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL,
    name VARCHAR(100) NOT NULL,
    price BIGINT NOT NULL,
    total_stock INTEGER NOT NULL,
    available_stock INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_ticket_types_event
        FOREIGN KEY (event_id)
        REFERENCES events(id),

    CONSTRAINT ticket_types_price_check
        CHECK (price >= 0),

    CONSTRAINT ticket_types_stock_check
        CHECK (
            total_stock >= 0
            AND available_stock >= 0
            AND available_stock <= total_stock
        ),

    CONSTRAINT ticket_types_event_name_unique
        UNIQUE (event_id, name)
);

CREATE INDEX idx_ticket_types_event_id
    ON ticket_types(event_id);
