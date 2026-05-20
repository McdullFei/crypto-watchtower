package model

import "time"

type MarketEvent struct {
	ID         string
	Exchange   string
	MarketType string
	Symbol     string
	EventType  string
	Side       string
	Price      float64
	Quantity   float64
	Notional   float64
	Metadata   map[string]any
	RawPayload []byte
	EventTime  time.Time
	CreatedAt  time.Time
}

func (e MarketEvent) TriggerBucket(alertType string) string {
	return e.Exchange + ":" + e.Symbol + ":" + alertType
}
