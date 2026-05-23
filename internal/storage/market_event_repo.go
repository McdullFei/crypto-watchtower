package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type MarketEventRepo struct {
	DB *pgxpool.Pool
}

func (r MarketEventRepo) Insert(ctx context.Context, event model.MarketEvent) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	raw := []byte("null")
	if len(event.RawPayload) > 0 {
		raw = event.RawPayload
	}
	_, err = r.DB.Exec(ctx, `
		INSERT INTO market_events
			(event_id, exchange, market_type, symbol, event_type, side, price, quantity, notional, metadata, raw_payload, event_time, created_at)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12,$13)
		ON CONFLICT (event_id) DO NOTHING
	`, event.ID, event.Exchange, event.MarketType, event.Symbol, event.EventType, event.Side, event.Price, event.Quantity, event.Notional, string(metadata), string(raw), event.EventTime, event.CreatedAt)
	return err
}

func (r MarketEventRepo) List(ctx context.Context, filter ListFilter) ([]model.MarketEvent, error) {
	query := `
		SELECT event_id, exchange, market_type, symbol, event_type, side, price, quantity, notional, metadata, event_time, created_at
		FROM market_events
		WHERE 1=1
	`
	args := make([]any, 0, 3)
	if filter.Symbol != "" {
		args = append(args, filter.Symbol)
		query += fmt.Sprintf(" AND symbol = $%d", len(args))
	}
	if filter.EventType != "" {
		args = append(args, filter.EventType)
		query += fmt.Sprintf(" AND event_type = $%d", len(args))
	}
	query += " ORDER BY event_time DESC"
	args = append(args, normalizedLimit(filter.Limit))
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.MarketEvent
	for rows.Next() {
		var item model.MarketEvent
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.Exchange, &item.MarketType, &item.Symbol, &item.EventType, &item.Side, &item.Price, &item.Quantity, &item.Notional, &metadataRaw, &item.EventTime, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(metadataRaw) > 0 && string(metadataRaw) != "null" {
			if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
				return nil, err
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r MarketEventRepo) CountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM market_events WHERE created_at >= $1`, since).Scan(&count)
	return count, err
}
