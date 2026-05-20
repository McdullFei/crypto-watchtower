package rule

import (
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestLargeTradeRuleTriggersWhenThresholdExceeded(t *testing.T) {
	rule := LargeTradeRule{Threshold: 100000}
	event := model.MarketEvent{
		ID:         "evt-1",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Notional:   150000,
		EventTime:  time.Now(),
	}

	alert, ok := rule.Evaluate(event)
	if !ok {
		t.Fatal("expected rule to trigger")
	}
	if alert.Type != "large_trade" {
		t.Fatalf("unexpected alert type: %s", alert.Type)
	}
}
