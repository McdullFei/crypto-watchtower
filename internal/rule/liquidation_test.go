package rule

import (
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestLiquidationRuleTriggersWhenThresholdExceeded(t *testing.T) {
	rule := LiquidationRule{Threshold: 100000}
	event := model.MarketEvent{
		ID:         "evt-2",
		Exchange:   "binance",
		MarketType: "futures",
		Symbol:     "ETHUSDT",
		EventType:  "liquidation",
		Notional:   120000,
		EventTime:  time.Now(),
	}

	alert, ok := rule.Evaluate(event)
	if !ok {
		t.Fatal("expected rule to trigger")
	}
	if alert.Type != "liquidation" {
		t.Fatalf("unexpected alert type: %s", alert.Type)
	}
}
