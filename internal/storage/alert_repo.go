package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type AlertRepo struct {
	DB *pgxpool.Pool
}

func (r AlertRepo) Insert(ctx context.Context, alert model.Alert) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO alerts
			(id, exchange, market_type, symbol, type, severity, title, message, event_id, rule_id, trigger_key, trigger_time, created_at)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO NOTHING
	`, alert.ID, alert.Exchange, alert.MarketType, alert.Symbol, alert.Type, alert.Severity, alert.Title, alert.Message, alert.EventID, alert.RuleID, alert.TriggerKey, alert.TriggerTime, alert.CreatedAt)
	return err
}
