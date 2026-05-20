package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
)

func TestNormalizeSpotAggTradeComputesNotional(t *testing.T) {
	raw := []byte(`{"e":"aggTrade","s":"BTCUSDT","a":1,"p":"100000.0","q":"2.0","m":false,"T":1710000000000}`)

	event, err := NormalizeAggTrade(raw, MarketTypeSpot)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if event.Notional != 200000 {
		t.Fatalf("unexpected notional: %f", event.Notional)
	}
	if event.MarketType != MarketTypeSpot {
		t.Fatalf("unexpected market type: %s", event.MarketType)
	}
}

func TestNormalizeLiquidationMapsSide(t *testing.T) {
	raw := []byte(`{"o":{"s":"ETHUSDT","S":"SELL","ap":"3821.2","q":"10","T":1710000001000}}`)

	event, err := NormalizeLiquidation(raw)
	if err != nil {
		t.Fatalf("normalize liquidation: %v", err)
	}

	if event.Side != "Long Liquidation" {
		t.Fatalf("unexpected side: %s", event.Side)
	}
}

func TestNormalizeFundingEventUsesRateFromPayload(t *testing.T) {
	event := NormalizeFunding("BTCUSDT", 0.12, 1710000002000)
	if event.EventType != "funding" {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	if event.Metadata["funding_rate"] != 0.12 {
		t.Fatalf("unexpected funding rate metadata: %v", event.Metadata["funding_rate"])
	}
}

func TestFundingFetcherFetchesRatesFromHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","lastFundingRate":"0.0012","time":1710000002000}]`))
	}))
	defer server.Close()

	bus := eventbus.New(1)
	fetcher := NewFundingFetcher(server.URL, []string{"BTCUSDT"}, bus)

	ch := bus.Subscribe(context.Background())
	if err := fetcher.Fetch(context.Background()); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	event := <-ch
	if event.Symbol != "BTCUSDT" {
		t.Fatalf("unexpected symbol: %s", event.Symbol)
	}
	if got := event.Metadata["funding_rate"]; got != 0.12 {
		t.Fatalf("unexpected funding rate: %v", got)
	}
}
