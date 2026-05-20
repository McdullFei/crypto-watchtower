package eventbus

import (
	"context"
	"sync"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type Bus struct {
	mu          sync.RWMutex
	buffer      int
	subscribers []chan model.MarketEvent
}

func New(buffer int) *Bus {
	if buffer <= 0 {
		buffer = 1
	}
	return &Bus{buffer: buffer}
}

func (b *Bus) Publish(ctx context.Context, event model.MarketEvent) error {
	b.mu.RLock()
	subs := append([]chan model.MarketEvent(nil), b.subscribers...)
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sub <- event:
		}
	}
	return nil
}

func (b *Bus) Subscribe(ctx context.Context) <-chan model.MarketEvent {
	ch := make(chan model.MarketEvent, b.buffer)

	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, sub := range b.subscribers {
			if sub == ch {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				break
			}
		}
		close(ch)
	}()

	return ch
}
