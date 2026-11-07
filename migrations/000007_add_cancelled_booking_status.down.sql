-- Refuse downgrade while cancelled bookings exist; keep their history intact.
ALTER TABLE bookings
    DROP CONSTRAINT bookings_status_check,
    ADD CONSTRAINT bookings_status_check
        CHECK (status IN ('pending', 'confirmed', 'expired'));
