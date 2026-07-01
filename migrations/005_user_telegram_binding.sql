CREATE TABLE IF NOT EXISTS telegram_binding_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_telegram_binding_tokens_token_hash
    ON telegram_binding_tokens(token_hash);

CREATE INDEX IF NOT EXISTS idx_telegram_binding_tokens_user_expires
    ON telegram_binding_tokens(user_id, expires_at DESC);
