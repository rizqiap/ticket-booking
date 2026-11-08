DROP INDEX IF EXISTS payments_midtrans_order_id_unique;
DROP INDEX IF EXISTS payments_booking_id_unique;

ALTER TABLE payments
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS redirect_url,
    DROP COLUMN IF EXISTS snap_token,
    DROP COLUMN IF EXISTS midtrans_order_id;
