package summary

import (
	"context"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/storage"
)

// TestAggregatorBuildsBoundedSnapshot verifies summary snapshots cap sample rows while keeping fetched counts.
//
// Author: monsterfei
// Date: 2026-06-30
func TestAggregatorBuildsBoundedSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	alerts := []model.Alert{
		{ID: "a1", Exchange: "binance", Symbol: "BTCUSDT", Type: "large_trade", Title: "BTC large trade", CreatedAt: now.Add(-time.Minute)},
		{ID: "a2", Exchange: "okx", Symbol: "ETHUSDT", Type: "funding_anomaly", Title: "ETH funding", CreatedAt: now.Add(-2 * time.Minute)},
	}
	events := []model.MarketEvent{
		{ID: "e1", Exchange: "binance", Symbol: "BTCUSDT", EventType: "agg_trade", Notional: 120000, EventTime: now.Add(-time.Minute)},
		{ID: "e2", Exchange: "okx", Symbol: "ETHUSDT", EventType: "funding", Metadata: map[string]any{"funding_rate": 0.12}, EventTime: now.Add(-2 * time.Minute)},
	}
	aggregator := NewAggregator(fakeAlertLister{items: alerts}, fakeEventLister{items: events}, Config{MaxItems: 1})

	snapshot, err := aggregator.Build(context.Background(), now.Add(-15*time.Minute), now)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if len(snapshot.Alerts) != 1 || len(snapshot.Events) != 1 {
		t.Fatalf("expected bounded snapshot, got %+v", snapshot)
	}
	if snapshot.AlertCount != 2 || snapshot.EventCount != 2 || snapshot.FundingCount != 1 {
		t.Fatalf("unexpected counts: %+v", snapshot)
	}
}

type fakeAlertLister struct {
	items []model.Alert
}

// List returns fake alerts for aggregator tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (f fakeAlertLister) List(_ context.Context, _ storage.ListFilter) ([]model.Alert, error) {
	return f.items, nil
}

type fakeEventLister struct {
	items []model.MarketEvent
}

// List returns fake market events for aggregator tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (f fakeEventLister) List(_ context.Context, _ storage.ListFilter) ([]model.MarketEvent, error) {
	return f.items, nil
}
