package collector

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type ExchangeCollector interface {
	Name() string
	Start(context.Context) error
	Subscribe([]string) error
}

type BinanceWSCollector struct {
	baseURL    string
	name       string
	marketType string
	bus        *eventbus.Bus
	symbols    []string
	dialer     *websocket.Dialer
}

func NewBinanceWSCollector(marketType string, baseURL string, bus *eventbus.Bus) *BinanceWSCollector {
	return &BinanceWSCollector{
		baseURL:    baseURL,
		name:       "binance-" + marketType,
		marketType: marketType,
		bus:        bus,
		dialer:     websocket.DefaultDialer,
	}
}

func (c *BinanceWSCollector) Name() string {
	return c.name
}

func (c *BinanceWSCollector) Start(ctx context.Context) error {
	if len(c.symbols) == 0 {
		return errors.New("no symbols subscribed")
	}
	streamURL, err := c.streamURL()
	if err != nil {
		return err
	}

	conn, _, err := c.dialer.DialContext(ctx, streamURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		event, ok, err := c.parseMessage(payload)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := c.bus.Publish(ctx, event); err != nil {
			return err
		}
	}
}

func (c *BinanceWSCollector) Subscribe(symbols []string) error {
	c.symbols = append([]string(nil), symbols...)
	return nil
}

func (c *BinanceWSCollector) streamURL() (string, error) {
	if c.baseURL == "" {
		return "", errors.New("base URL is required")
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	streams := c.streamNames()
	if len(streams) == 0 {
		return "", errors.New("no streams configured")
	}
	path := strings.TrimSuffix(u.Path, "/")
	path = strings.TrimSuffix(path, "/ws")
	path = strings.TrimSuffix(path, "/stream")
	u.Path = path + "/stream"
	query := u.Query()
	query.Set("streams", strings.Join(streams, "/"))
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (c *BinanceWSCollector) streamNames() []string {
	streams := make([]string, 0, len(c.symbols)*2)
	for _, symbol := range c.symbols {
		lower := strings.ToLower(symbol)
		streams = append(streams, lower+"@aggTrade")
		if c.marketType == MarketTypeFutures {
			streams = append(streams, lower+"@forceOrder")
		}
	}
	return streams
}

type combinedStreamEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

type eventTypeEnvelope struct {
	EventType string `json:"e"`
}

func (c *BinanceWSCollector) parseMessage(payload []byte) (model.MarketEvent, bool, error) {
	var envelope combinedStreamEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return model.MarketEvent{}, false, err
	}
	if len(envelope.Data) == 0 {
		return model.MarketEvent{}, false, nil
	}

	var evt eventTypeEnvelope
	if err := json.Unmarshal(envelope.Data, &evt); err != nil {
		return model.MarketEvent{}, false, err
	}

	switch evt.EventType {
	case "aggTrade":
		event, err := NormalizeAggTrade(envelope.Data, c.marketType)
		return event, err == nil, err
	case "forceOrder":
		event, err := NormalizeLiquidation(envelope.Data)
		return event, err == nil, err
	default:
		return model.MarketEvent{}, false, nil
	}
}
