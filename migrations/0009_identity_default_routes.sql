ALTER TABLE users
    ADD COLUMN IF NOT EXISTS preferred_user_route TEXT NULL,
    ADD COLUMN IF NOT EXISTS preferred_admin_route TEXT NULL;

ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS default_user_route TEXT NULL,
    ADD COLUMN IF NOT EXISTS default_admin_route TEXT NULL;

ALTER TABLE role_bindings
    ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 0;
