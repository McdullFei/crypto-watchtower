package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestBusPublishesEventsToSubscribers(t *testing.T) {
	bus := New(8)
	ch := bus.Subscribe(context.Background())

	event := model.MarketEvent{ID: "evt-1", Symbol: "BTCUSDT"}
	if err := bus.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-ch:
		if got.ID != event.ID {
			t.Fatalf("unexpected event id: %s", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}
