package rule

import (
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestFundingRuleTriggersOnAbsoluteThreshold(t *testing.T) {
	rule := FundingRule{AbsThreshold: 0.08}
	event := model.MarketEvent{
		ID:         "evt-3",
		Exchange:   "binance",
		MarketType: "futures",
		Symbol:     "BTCUSDT",
		EventType:  "funding",
		Metadata: map[string]any{
			"funding_rate": 0.12,
		},
		EventTime: time.Now(),
	}

	alert, ok := rule.Evaluate(event)
	if !ok {
		t.Fatal("expected rule to trigger")
	}
	if alert.Type != "funding_anomaly" {
		t.Fatalf("unexpected alert type: %s", alert.Type)
	}
}
