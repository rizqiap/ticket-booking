CREATE TABLE bookings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    ticket_type_id BIGINT NOT NULL REFERENCES ticket_types(id),
    quantity INTEGER NOT NULL,
    total_price BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT bookings_quantity_check
        CHECK (quantity BETWEEN 1 AND 4),

    CONSTRAINT bookings_price_check
        CHECK (total_price >= 0),

    CONSTRAINT bookings_status_check
        CHECK (status IN ('pending', 'confirmed', 'expired')),

    CONSTRAINT bookings_idempotency_unique
        UNIQUE (user_id, idempotency_key)
);

CREATE INDEX idx_bookings_pending_expiration
    ON bookings(expires_at)
    WHERE status = 'pending';
