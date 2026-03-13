ALTER TABLE users
    ADD COLUMN IF NOT EXISTS authentication_subject TEXT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_authentication_subject
    ON users (authentication_subject)
    WHERE authentication_subject IS NOT NULL;
