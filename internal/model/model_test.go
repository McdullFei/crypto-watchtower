package model

import "testing"

func TestAlertTriggerBucketIncludesExchangeSymbolType(t *testing.T) {
	alert := Alert{
		Exchange: "binance",
		Symbol:   "BTCUSDT",
		Type:     "large_trade",
	}

	if got := alert.TriggerBucket(); got != "binance:BTCUSDT:large_trade" {
		t.Fatalf("unexpected trigger bucket: %s", got)
	}
}
