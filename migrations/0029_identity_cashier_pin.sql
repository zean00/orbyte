BEGIN;

ALTER TABLE user_credentials
    ADD COLUMN IF NOT EXISTS cashier_pin_hash TEXT NULL,
    ADD COLUMN IF NOT EXISTS cashier_pin_changed_at TIMESTAMPTZ NULL;

COMMIT;
