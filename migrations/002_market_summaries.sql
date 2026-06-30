CREATE TABLE IF NOT EXISTS market_summaries (
    id VARCHAR(128) PRIMARY KEY,
    window_from TIMESTAMP NOT NULL,
    window_to TIMESTAMP NOT NULL,
    provider VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_summaries_window
    ON market_summaries(window_to DESC);
