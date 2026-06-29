package main

import (
	"context"
	"testing"

	"github.com/renfei198727/crypto-watchtower/internal/collector"
	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// TestBuildMarketCollectorsIncludesOKXWhenEnabled verifies runtime collector wiring.
//
// Author: monsterfei
// Date: 2026-06-29
func TestBuildMarketCollectorsIncludesOKXWhenEnabled(t *testing.T) {
	cfg := validServerConfig()
	cfg.OKX.Enabled = true
	cfg.OKX.PublicWSBaseURL = "wss://ws.okx.com:8443/ws/v5/public"
	cfg.OKX.Symbols = []string{"BTCUSDT"}

	collectors, err := buildMarketCollectors(cfg, eventbus.New(1), collector.NewOKXInstrumentStore([]collector.OKXInstrument{
		{InstID: "BTC-USDT", InstType: "SPOT"},
		{InstID: "BTC-USDT-SWAP", InstType: "SWAP", CtVal: 0.01, CtValCcy: "BTC", SettleCcy: "USDT"},
	}))
	if err != nil {
		t.Fatalf("build collectors: %v", err)
	}
	if len(collectors) != 3 {
		t.Fatalf("expected binance spot, binance futures, and OKX collectors, got %d", len(collectors))
	}
	if collectors[2].Status().Name != "okx-public" {
		t.Fatalf("expected OKX collector last, got %+v", collectors[2].Status())
	}
}

// TestBuildMarketCollectorsSkipsOKXWhenDisabled verifies existing Binance-only default.
//
// Author: monsterfei
// Date: 2026-06-29
func TestBuildMarketCollectorsSkipsOKXWhenDisabled(t *testing.T) {
	cfg := validServerConfig()
	cfg.OKX.Enabled = false

	collectors, err := buildMarketCollectors(cfg, eventbus.New(1), collector.NewOKXInstrumentStore(nil))
	if err != nil {
		t.Fatalf("build collectors: %v", err)
	}
	if len(collectors) != 2 {
		t.Fatalf("expected only Binance collectors, got %d", len(collectors))
	}
}

// TestBuildMarketCollectorsAllowsOKXOnly verifies Binance can be disabled for OKX-only smoke.
//
// Author: __AUTHOR__
// Date: 2026-06-29
func TestBuildMarketCollectorsAllowsOKXOnly(t *testing.T) {
	cfg := validServerConfig()
	cfg.Binance.Enabled = false
	cfg.OKX.Enabled = true
	cfg.OKX.PublicWSBaseURL = "wss://ws.okx.com:8443/ws/v5/public"
	cfg.OKX.Symbols = []string{"BTCUSDT"}

	collectors, err := buildMarketCollectors(cfg, eventbus.New(1), collector.NewOKXInstrumentStore([]collector.OKXInstrument{
		{InstID: "BTC-USDT", InstType: "SPOT"},
		{InstID: "BTC-USDT-SWAP", InstType: "SWAP", CtVal: 0.01, CtValCcy: "BTC", SettleCcy: "USDT"},
	}))
	if err != nil {
		t.Fatalf("build collectors: %v", err)
	}
	if len(collectors) != 1 {
		t.Fatalf("expected only OKX collector, got %d", len(collectors))
	}
	if collectors[0].Status().Name != "okx-public" {
		t.Fatalf("expected OKX collector, got %+v", collectors[0].Status())
	}
}

// TestBuildNotificationSendersAddsWebhookWhenEnabled verifies optional webhook runtime wiring.
//
// Author: __AUTHOR__
// Date: 2026-06-29
func TestBuildNotificationSendersAddsWebhookWhenEnabled(t *testing.T) {
	cfg := validServerConfig()
	cfg.Telegram.Enabled = true
	cfg.Webhook.Enabled = true
	cfg.Webhook.URL = "https://discord.example/webhook"
	cfg.Webhook.Channel = "discord"
	cfg.Webhook.TimeoutSec = 7

	senders := buildNotificationSenders(cfg, serverStubSender{})

	if len(senders) != 2 {
		t.Fatalf("expected telegram and webhook senders, got %d", len(senders))
	}
	if senders[0].Name() != "telegram" || senders[0].Target() != "default" {
		t.Fatalf("unexpected telegram sender: %s %s", senders[0].Name(), senders[0].Target())
	}
	if senders[1].Name() != "discord" || senders[1].Target() != "https://discord.example/webhook" {
		t.Fatalf("unexpected webhook sender: %s %s", senders[1].Name(), senders[1].Target())
	}
}

type serverStubSender struct{}

func (serverStubSender) Send(context.Context, model.Alert) error {
	return nil
}

// validServerConfig returns a minimal config for server wiring tests.
//
// Author: monsterfei
// Date: 2026-06-29
func validServerConfig() config.Config {
	cfg := config.Config{}
	cfg.Binance.Enabled = true
	cfg.Binance.SpotWSBaseURL = "wss://stream.binance.com:9443/ws"
	cfg.Binance.FuturesWSBaseURL = "wss://fstream.binance.com/ws"
	cfg.Binance.FuturesRESTBaseURL = "https://fapi.binance.com"
	cfg.Binance.Symbols = []string{"BTCUSDT"}
	cfg.API.BearerToken = "token"
	cfg.Postgres.DSN = "postgres://example"
	cfg.Redis.Addr = "localhost:6379"
	return cfg
}
