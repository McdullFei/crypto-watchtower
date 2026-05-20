CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255),
    telegram_chat_id VARCHAR(128),
    plan VARCHAR(32) DEFAULT 'free',
    status VARCHAR(32) DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_telegram_chat_id
    ON users(telegram_chat_id);

CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    scope VARCHAR(32) NOT NULL DEFAULT 'system',
    exchange VARCHAR(32) NOT NULL DEFAULT 'binance',
    symbol VARCHAR(32) NOT NULL,
    rule_type VARCHAR(64) NOT NULL,
    threshold NUMERIC(24, 8) NOT NULL,
    window_sec INTEGER NOT NULL DEFAULT 60,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_lookup
    ON alert_rules(exchange, symbol, rule_type, enabled);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_rules_system_unique
    ON alert_rules(scope, exchange, symbol, rule_type)
    WHERE user_id IS NULL;

CREATE TABLE IF NOT EXISTS market_events (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(128) NOT NULL UNIQUE,
    exchange VARCHAR(32) NOT NULL,
    market_type VARCHAR(32) NOT NULL,
    symbol VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    side VARCHAR(32),
    price NUMERIC(24, 8),
    quantity NUMERIC(24, 8),
    notional NUMERIC(24, 8),
    metadata JSONB,
    raw_payload JSONB,
    event_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_events_symbol_time
    ON market_events(symbol, event_time DESC);

CREATE TABLE IF NOT EXISTS alerts (
    id VARCHAR(128) PRIMARY KEY,
    exchange VARCHAR(32) NOT NULL,
    market_type VARCHAR(32) NOT NULL,
    symbol VARCHAR(32) NOT NULL,
    type VARCHAR(64) NOT NULL,
    severity VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    event_id VARCHAR(128) NOT NULL,
    rule_id VARCHAR(128),
    trigger_key VARCHAR(255) NOT NULL,
    trigger_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_symbol_time
    ON alerts(symbol, created_at DESC);

CREATE TABLE IF NOT EXISTS notification_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    alert_id VARCHAR(128) NOT NULL,
    channel VARCHAR(32) NOT NULL,
    target VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_logs_alert_id
    ON notification_logs(alert_id);
