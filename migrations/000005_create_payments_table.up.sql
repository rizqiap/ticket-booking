CREATE TABLE payments (
    id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL REFERENCES bookings(id),
    amount BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT payments_amount_check
        CHECK (amount >= 0),

    CONSTRAINT payments_status_check
        CHECK (status IN ('pending', 'success', 'failed'))
);

CREATE INDEX idx_payments_booking_id
    ON payments(booking_id);

CREATE UNIQUE INDEX idx_payments_one_success_per_booking
    ON payments(booking_id)
    WHERE status = 'success';