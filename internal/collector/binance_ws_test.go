package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
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

func TestStartReconnectsAfterReadFailure(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		current := connections.Add(1)
		if current == 1 {
			_ = conn.Close()
			return
		}

		msg := `{"stream":"btcusdt@aggTrade","data":{"e":"aggTrade","s":"BTCUSDT","a":2,"p":"100000.0","q":"2.0","m":false,"T":1710000000001}}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("write message: %v", err)
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	bus := eventbus.New(1)
	collector := NewBinanceWSCollector(MarketTypeSpot, wsURL, bus)
	collector.SetReconnectBackoff(5*time.Millisecond, 10*time.Millisecond)
	if err := collector.Subscribe([]string{"BTCUSDT"}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = collector.Start(ctx)
	}()

	select {
	case event := <-bus.Subscribe(ctx):
		if event.ID != "binance-spot-agg-2" {
			t.Fatalf("unexpected event after reconnect: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event after reconnect")
	}

	status := collector.Status()
	if status.Reconnects < 1 {
		t.Fatalf("expected reconnect count, got %+v", status)
	}
}

func TestStartReconnectsAfterReadDeadline(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		current := connections.Add(1)
		if current == 1 {
			time.Sleep(70 * time.Millisecond)
			return
		}

		msg := `{"stream":"btcusdt@aggTrade","data":{"e":"aggTrade","s":"BTCUSDT","a":3,"p":"100000.0","q":"2.0","m":false,"T":1710000000002}}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("write message: %v", err)
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	bus := eventbus.New(1)
	collector := NewBinanceWSCollector(MarketTypeSpot, wsURL, bus)
	collector.SetReconnectBackoff(5*time.Millisecond, 10*time.Millisecond)
	collector.SetHeartbeat(30*time.Millisecond, 0, 10*time.Millisecond)
	if err := collector.Subscribe([]string{"BTCUSDT"}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = collector.Start(ctx)
	}()

	select {
	case event := <-bus.Subscribe(ctx):
		if event.ID != "binance-spot-agg-3" {
			t.Fatalf("unexpected event after reconnect: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event after read deadline reconnect")
	}

	status := collector.Status()
	if status.Reconnects < 1 {
		t.Fatalf("expected reconnect count, got %+v", status)
	}
}

func TestStartKeepsConnectionAliveWithPingPong(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		time.Sleep(120 * time.Millisecond)
		msg := `{"stream":"btcusdt@aggTrade","data":{"e":"aggTrade","s":"BTCUSDT","a":4,"p":"100000.0","q":"2.0","m":false,"T":1710000000003}}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("write message: %v", err)
		}

		<-done
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	bus := eventbus.New(1)
	collector := NewBinanceWSCollector(MarketTypeSpot, wsURL, bus)
	collector.SetHeartbeat(40*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond)
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
		if event.ID != "binance-spot-agg-4" {
			t.Fatalf("unexpected event after ping/pong: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event after ping/pong keepalive")
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
