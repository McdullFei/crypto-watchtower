CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_rules_user_unique
    ON alert_rules(user_id, scope, exchange, symbol, rule_type)
    WHERE user_id IS NOT NULL;
