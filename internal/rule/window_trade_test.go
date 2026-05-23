package rule

import (
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestWindowLargeTradeRuleTriggersOnThresholdCross(t *testing.T) {
	engine := NewEngine(Config{
		LargeTradeThreshold:       100000,
		LargeTradeWindowThreshold: 250000,
		LargeTradeWindowSec:       60,
	})

	baseTime := time.Unix(1710000000, 0).UTC()
	first := engine.Evaluate(model.MarketEvent{
		ID:         "event-1",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Side:       "Aggressive Buy",
		Price:      100000,
		Quantity:   1,
		Notional:   100000,
		EventTime:  baseTime,
	})
	if len(first) != 1 || first[0].Type != "large_trade" {
		t.Fatalf("expected single trade alert only, got %+v", first)
	}

	second := engine.Evaluate(model.MarketEvent{
		ID:         "event-2",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Side:       "Aggressive Buy",
		Price:      100000,
		Quantity:   1.6,
		Notional:   160000,
		EventTime:  baseTime.Add(20 * time.Second),
	})
	if len(second) != 2 {
		t.Fatalf("expected single trade + window alert, got %+v", second)
	}
	if second[1].Type != "large_trade_window" {
		t.Fatalf("expected window alert, got %+v", second[1])
	}
}

func TestWindowLargeTradeRuleSlidesOutOldTrades(t *testing.T) {
	engine := NewEngine(Config{
		LargeTradeThreshold:       500000,
		LargeTradeWindowThreshold: 250000,
		LargeTradeWindowSec:       60,
	})

	baseTime := time.Unix(1710000000, 0).UTC()
	if alerts := engine.Evaluate(model.MarketEvent{
		ID:         "event-1",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Notional:   150000,
		EventTime:  baseTime,
	}); len(alerts) != 0 {
		t.Fatalf("expected no alert, got %+v", alerts)
	}

	if alerts := engine.Evaluate(model.MarketEvent{
		ID:         "event-2",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Notional:   120000,
		EventTime:  baseTime.Add(61 * time.Second),
	}); len(alerts) != 0 {
		t.Fatalf("expected no window alert after first trade expired, got %+v", alerts)
	}
}
