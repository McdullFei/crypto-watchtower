package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
)

func TestBuildCombinedStreamURLForSpot(t *testing.T) {
	bus := eventbus.New(1)
	collector := NewBinanceWSCollector(MarketTypeSpot, "wss://stream.binance.com:9443/ws", bus)
	if err := collector.Subscribe([]string{"BTCUSDT", "ETHUSDT"}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	got, err := collector.streamURL()
	if err != nil {
		t.Fatalf("streamURL: %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	streams := parsed.Query().Get("streams")
	if !strings.Contains(streams, "btcusdt@aggTrade") || !strings.Contains(streams, "ethusdt@aggTrade") {
		t.Fatalf("unexpected stream url: %s", got)
	}
	if strings.Contains(streams, "forceOrder") {
		t.Fatalf("spot collector should not include forceOrder stream: %s", got)
	}
}

func TestBuildCombinedStreamURLForFuturesIncludesForceOrder(t *testing.T) {
	bus := eventbus.New(1)
	collector := NewBinanceWSCollector(MarketTypeFutures, "wss://fstream.binance.com/ws", bus)
	if err := collector.Subscribe([]string{"BTCUSDT"}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	got, err := collector.streamURL()
	if err != nil {
		t.Fatalf("streamURL: %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	streams := parsed.Query().Get("streams")
	if !strings.Contains(streams, "btcusdt@aggTrade") || !strings.Contains(streams, "btcusdt@forceOrder") {
		t.Fatalf("unexpected futures stream url: %s", got)
	}
}

func TestStartPublishesAggTradeEventFromCombinedStream(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		msg := `{"stream":"btcusdt@aggTrade","data":{"e":"aggTrade","s":"BTCUSDT","a":1,"p":"100000.0","q":"2.0","m":false,"T":1710000000000}}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("write message: %v", err)
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	bus := eventbus.New(1)
	collector := NewBinanceWSCollector(MarketTypeSpot, wsURL, bus)
	if err := collector.Subscribe([]string{"BTCUSDT"}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- collector.Start(ctx)
	}()

	select {
	case event := <-bus.Subscribe(ctx):
		if event.Symbol != "BTCUSDT" || event.EventType != "agg_trade" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("unexpected start error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for collector shutdown")
	}
}
