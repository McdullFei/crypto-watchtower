package collector

import (
	"math"
	"testing"
	"time"
)

// TestOKXNormalizeTrades maps OKX trades messages into market events.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXNormalizeTrades(t *testing.T) {
	raw := []byte(`{
		"arg": {"channel": "trades", "instId": "BTC-USDT"},
		"data": [
			{
				"instId": "BTC-USDT",
				"tradeId": "130639474",
				"px": "42219.9",
				"sz": "0.12060306",
				"side": "buy",
				"ts": "1630048897897",
				"count": "3",
				"source": "0",
				"seqId": 1234
			}
		]
	}`)
	store := NewOKXInstrumentStore([]OKXInstrument{{InstID: "BTC-USDT", InstType: "SPOT"}})

	events, err := NormalizeOKXTrades(raw, store)
	if err != nil {
		t.Fatalf("normalize OKX trades: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	event := events[0]
	if event.Exchange != "okx" || event.MarketType != MarketTypeSpot || event.Symbol != "BTCUSDT" {
		t.Fatalf("unexpected routing fields: %#v", event)
	}
	if event.EventType != "agg_trade" || event.Side != "Aggressive Buy" {
		t.Fatalf("unexpected event fields: %#v", event)
	}
	if event.ID != "okx-spot-trades-BTC-USDT-130639474" {
		t.Fatalf("unexpected event id %q", event.ID)
	}
	if math.Abs(event.Notional-5091.849132894) > 0.000000001 {
		t.Fatalf("unexpected notional %v", event.Notional)
	}
	if event.EventTime != time.UnixMilli(1630048897897).UTC() {
		t.Fatalf("unexpected event time %s", event.EventTime)
	}
	if event.Metadata["native_inst_id"] != "BTC-USDT" || event.Metadata["count"] != "3" {
		t.Fatalf("unexpected metadata %#v", event.Metadata)
	}
}

// TestOKXNormalizeLiquidations maps OKX liquidation details into events.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXNormalizeLiquidations(t *testing.T) {
	raw := []byte(`{
		"arg": {"channel": "liquidation-orders", "instType": "SWAP"},
		"data": [
			{
				"instFamily": "BTC-USDT",
				"instId": "BTC-USDT-SWAP",
				"instType": "SWAP",
				"details": [
					{
						"bkLoss": "0",
						"bkPx": "50000",
						"ccy": "",
						"posSide": "long",
						"side": "sell",
						"sz": "4",
						"ts": "1692266434010"
					}
				]
			}
		]
	}`)
	store := NewOKXInstrumentStore([]OKXInstrument{{
		InstID: "BTC-USDT-SWAP", InstType: "SWAP", CtVal: 0.01, CtValCcy: "BTC", SettleCcy: "USDT",
	}})

	events, err := NormalizeOKXLiquidations(raw, store)
	if err != nil {
		t.Fatalf("normalize OKX liquidations: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	event := events[0]
	if event.Exchange != "okx" || event.MarketType != MarketTypeFutures || event.Symbol != "BTCUSDT" {
		t.Fatalf("unexpected routing fields: %#v", event)
	}
	if event.EventType != "liquidation" || event.Side != "Long Liquidation" {
		t.Fatalf("unexpected liquidation fields: %#v", event)
	}
	if event.Notional != 2000 {
		t.Fatalf("unexpected notional %v", event.Notional)
	}
	if event.Metadata["pos_side"] != "long" || event.Metadata["native_inst_id"] != "BTC-USDT-SWAP" {
		t.Fatalf("unexpected metadata %#v", event.Metadata)
	}
}

// TestOKXNormalizeFunding maps OKX funding updates into percentage metadata.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXNormalizeFunding(t *testing.T) {
	raw := []byte(`{
		"arg": {"channel": "funding-rate", "instId": "BTC-USDT-SWAP"},
		"data": [
			{
				"formulaType": "noRate",
				"fundingRate": "0.0001875391284828",
				"fundingTime": "1700726400000",
				"instId": "BTC-USDT-SWAP",
				"instType": "SWAP",
				"method": "current_period",
				"nextFundingTime": "1700755200000",
				"settFundingRate": "0.0001699799259033",
				"settState": "settled",
				"ts": "1700724675402"
			}
		]
	}`)

	events, err := NormalizeOKXFunding(raw)
	if err != nil {
		t.Fatalf("normalize OKX funding: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	event := events[0]
	if event.Exchange != "okx" || event.MarketType != MarketTypeFutures || event.Symbol != "BTCUSDT" {
		t.Fatalf("unexpected routing fields: %#v", event)
	}
	if event.EventType != "funding" {
		t.Fatalf("unexpected event type %s", event.EventType)
	}
	if math.Abs(event.Metadata["funding_rate"].(float64)-0.01875391284828) > 0.00000000000001 {
		t.Fatalf("unexpected funding rate %v", event.Metadata["funding_rate"])
	}
	if event.EventTime != time.UnixMilli(1700726400000).UTC() {
		t.Fatalf("unexpected event time %s", event.EventTime)
	}
	if event.Metadata["next_funding_time"] != int64(1700755200000) || event.Metadata["sett_state"] != "settled" {
		t.Fatalf("unexpected metadata %#v", event.Metadata)
	}
}
