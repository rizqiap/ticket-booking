CREATE TABLE payment_notifications (
    id BIGSERIAL PRIMARY KEY,

    payment_id BIGINT NOT NULL
        REFERENCES payments(id),

    transaction_id TEXT NOT NULL,
    transaction_status TEXT NOT NULL,
    status_code TEXT NOT NULL,
    fraud_status TEXT NOT NULL DEFAULT '',

    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,

    CONSTRAINT payment_notification_unique UNIQUE (
        payment_id,
        transaction_id,
        transaction_status,
        status_code,
        fraud_status
    )
);

CREATE INDEX idx_payment_notifications_unprocessed
ON payment_notifications (id)
WHERE processed_at IS NULL;