ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS midtrans_order_id TEXT,
    ADD COLUMN IF NOT EXISTS snap_token TEXT,
    ADD COLUMN IF NOT EXISTS redirect_url TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS payments_booking_id_unique
    ON payments (booking_id);

CREATE UNIQUE INDEX IF NOT EXISTS payments_midtrans_order_id_unique
    ON payments (midtrans_order_id);
