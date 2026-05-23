package storage

import (
	"context"
	"fmt"
	"time"

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

func (r AlertRepo) List(ctx context.Context, filter ListFilter) ([]model.Alert, error) {
	query := `
		SELECT id, exchange, market_type, symbol, type, severity, title, message, event_id, rule_id, trigger_key, trigger_time, created_at
		FROM alerts
		WHERE 1=1
	`
	args := make([]any, 0, 3)
	if filter.Symbol != "" {
		args = append(args, filter.Symbol)
		query += fmt.Sprintf(" AND symbol = $%d", len(args))
	}
	if filter.RuleType != "" {
		args = append(args, filter.RuleType)
		query += fmt.Sprintf(" AND type = $%d", len(args))
	}
	query += " ORDER BY created_at DESC"
	args = append(args, normalizedLimit(filter.Limit))
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Alert
	for rows.Next() {
		var item model.Alert
		if err := rows.Scan(&item.ID, &item.Exchange, &item.MarketType, &item.Symbol, &item.Type, &item.Severity, &item.Title, &item.Message, &item.EventID, &item.RuleID, &item.TriggerKey, &item.TriggerTime, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r AlertRepo) CountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM alerts WHERE created_at >= $1`, since).Scan(&count)
	return count, err
}

func (r AlertRepo) LatestCreatedAt(ctx context.Context) (*time.Time, error) {
	var value *time.Time
	err := r.DB.QueryRow(ctx, `SELECT MAX(created_at) FROM alerts`).Scan(&value)
	return value, err
}
