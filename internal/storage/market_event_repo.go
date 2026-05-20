package storage

import (
	"context"
	"encoding/json"

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
