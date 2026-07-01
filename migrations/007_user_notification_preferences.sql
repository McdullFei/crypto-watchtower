ALTER TABLE users
	ADD COLUMN IF NOT EXISTS telegram_quiet_hours_enabled BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN IF NOT EXISTS telegram_quiet_hours_start TEXT NOT NULL DEFAULT '22:00',
	ADD COLUMN IF NOT EXISTS telegram_quiet_hours_end TEXT NOT NULL DEFAULT '08:00',
	ADD COLUMN IF NOT EXISTS telegram_quiet_hours_timezone TEXT NOT NULL DEFAULT 'UTC',
	ADD COLUMN IF NOT EXISTS telegram_digest_enabled BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN IF NOT EXISTS telegram_digest_interval_min INTEGER NOT NULL DEFAULT 60;

CREATE INDEX IF NOT EXISTS idx_users_telegram_quiet_hours_enabled
	ON users (telegram_quiet_hours_enabled)
	WHERE telegram_quiet_hours_enabled = TRUE;

CREATE INDEX IF NOT EXISTS idx_users_telegram_digest_enabled
	ON users (telegram_digest_enabled)
	WHERE telegram_digest_enabled = TRUE;
