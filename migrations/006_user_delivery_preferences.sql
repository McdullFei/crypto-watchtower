ALTER TABLE users
    ADD COLUMN IF NOT EXISTS telegram_delivery_enabled BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_users_telegram_delivery_enabled
    ON users (telegram_delivery_enabled)
    WHERE telegram_delivery_enabled = TRUE;
