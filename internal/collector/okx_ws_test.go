package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
)

// TestOKXWSStartSubscribesAndPublishesTrade verifies the collector subscription loop.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXWSStartSubscribesAndPublishesTrade(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		_, subscribeRaw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read subscribe: %v", err)
		}
		var subscribe okxSubscribeRequest
		if err := json.Unmarshal(subscribeRaw, &subscribe); err != nil {
			t.Fatalf("decode subscribe: %v", err)
		}
		if subscribe.Operation != "subscribe" {
			t.Fatalf("unexpected operation %s", subscribe.Operation)
		}
		assertOKXArg(t, subscribe.Args, "trades", "BTC-USDT", "")
		assertOKXArg(t, subscribe.Args, "trades", "BTC-USDT-SWAP", "")
		assertOKXArg(t, subscribe.Args, "funding-rate", "BTC-USDT-SWAP", "")
		assertOKXArg(t, subscribe.Args, "liquidation-orders", "", "SWAP")

		msg := `{
			"arg": {"channel": "trades", "instId": "BTC-USDT"},
			"data": [
				{"instId":"BTC-USDT","tradeId":"1","px":"100000","sz":"2","side":"buy","ts":"1710000000000","count":"1","source":"0","seqId":1}
			]
		}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("write trade: %v", err)
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	bus := eventbus.New(1)
	store := NewOKXInstrumentStore([]OKXInstrument{
		{InstID: "BTC-USDT", InstType: "SPOT"},
		{InstID: "BTC-USDT-SWAP", InstType: "SWAP", CtVal: 0.01, CtValCcy: "BTC", SettleCcy: "USDT"},
	})
	collector := NewOKXWSCollector(wsURL, bus, store)
	collector.SetReconnectBackoff(5*time.Millisecond, 10*time.Millisecond)
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
		if event.Exchange != "okx" || event.Symbol != "BTCUSDT" || event.EventType != "agg_trade" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for OKX event")
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

// TestOKXWSRecordsErrorAck verifies provider subscription errors appear in status.
//
// Author: monsterfei
// Date: 2026-06-29
func TestOKXWSRecordsErrorAck(t *testing.T) {
	collector := NewOKXWSCollector("wss://example/ws", eventbus.New(1), NewOKXInstrumentStore(nil))

	events, err := collector.handleMessage([]byte(`{"event":"error","code":"64003","msg":"fee tier unavailable"}`))
	if err != nil {
		t.Fatalf("handle error ack: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}
	if got := collector.Status().LastError; !strings.Contains(got, "64003") || !strings.Contains(got, "fee tier unavailable") {
		t.Fatalf("expected recorded OKX error, got %q", got)
	}
}

func assertOKXArg(t *testing.T, args []okxChannelArg, channel string, instID string, instType string) {
	t.Helper()
	for _, arg := range args {
		if arg.Channel == channel && arg.InstID == instID && arg.InstType == instType {
			return
		}
	}
	t.Fatalf("missing subscription arg channel=%s instID=%s instType=%s in %#v", channel, instID, instType, args)
}
