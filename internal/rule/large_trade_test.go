package rule

import (
	"strings"
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
	for _, expected := range []string{"交易所: binance", "交易对: BTCUSDT", "规则: large_trade", "阈值: 100000.00 USDT", "成交额: 150000.00 USDT"} {
		if !strings.Contains(alert.Message, expected) {
			t.Fatalf("expected alert message to contain %q, got %q", expected, alert.Message)
		}
	}
}
