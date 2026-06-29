package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOKXInstrumentSymbolMapping verifies Binance-style symbols map to OKX instruments.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXInstrumentSymbolMapping(t *testing.T) {
	if got := OKXSpotInstID("BTCUSDT"); got != "BTC-USDT" {
		t.Fatalf("expected BTC-USDT, got %q", got)
	}
	if got := OKXSwapInstID("BTCUSDT"); got != "BTC-USDT-SWAP" {
		t.Fatalf("expected BTC-USDT-SWAP, got %q", got)
	}
}

// TestOKXInstrumentNotional verifies spot and swap notional calculations.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXInstrumentNotional(t *testing.T) {
	store := NewOKXInstrumentStore([]OKXInstrument{
		{InstID: "BTC-USDT", InstType: "SPOT"},
		{InstID: "BTC-USDT-SWAP", InstType: "SWAP", CtVal: 0.01, CtValCcy: "BTC", SettleCcy: "USDT"},
	})

	spot, err := store.Notional("BTC-USDT", 50000, 0.2)
	if err != nil {
		t.Fatalf("spot notional: %v", err)
	}
	if spot != 10000 {
		t.Fatalf("expected spot notional 10000, got %v", spot)
	}

	swap, err := store.Notional("BTC-USDT-SWAP", 50000, 3)
	if err != nil {
		t.Fatalf("swap notional: %v", err)
	}
	if swap != 1500 {
		t.Fatalf("expected swap notional 1500, got %v", swap)
	}
}

// TestOKXInstrumentFetcherParsesPublicInstruments verifies REST metadata parsing.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXInstrumentFetcherParsesPublicInstruments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/public/instruments" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("instType") != "SWAP" {
			t.Fatalf("unexpected instType %s", r.URL.Query().Get("instType"))
		}
		_, _ = w.Write([]byte(`{
			"code": "0",
			"msg": "",
			"data": [
				{
					"instId": "BTC-USDT-SWAP",
					"instType": "SWAP",
					"ctVal": "0.01",
					"ctValCcy": "BTC",
					"settleCcy": "USDT"
				}
			]
		}`))
	}))
	defer server.Close()

	fetcher := NewOKXInstrumentFetcher(server.URL, server.Client())
	instruments, err := fetcher.Fetch(context.Background(), "SWAP")
	if err != nil {
		t.Fatalf("fetch instruments: %v", err)
	}
	if len(instruments) != 1 {
		t.Fatalf("expected one instrument, got %d", len(instruments))
	}
	if instruments[0].InstID != "BTC-USDT-SWAP" || instruments[0].CtVal != 0.01 {
		t.Fatalf("unexpected instrument %#v", instruments[0])
	}
}
