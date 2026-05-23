package collector

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type ExchangeCollector interface {
	Name() string
	Start(context.Context) error
	Subscribe([]string) error
}

type Status struct {
	Name          string
	Connected     bool
	Reconnects    int64
	LastEventAt   time.Time
	LastError     string
	Subscribed    []string
	LastConnectAt time.Time
}

type BinanceWSCollector struct {
	baseURL       string
	name          string
	marketType    string
	bus           *eventbus.Bus
	symbols       []string
	dialer        *websocket.Dialer
	minBackoff    time.Duration
	maxBackoff    time.Duration
	readTimeout   time.Duration
	pingInterval  time.Duration
	writeTimeout  time.Duration
	mu            sync.RWMutex
	connected     bool
	reconnects    int64
	lastEventAt   time.Time
	lastError     string
	lastConnectAt time.Time
}

func NewBinanceWSCollector(marketType string, baseURL string, bus *eventbus.Bus) *BinanceWSCollector {
	return &BinanceWSCollector{
		baseURL:      baseURL,
		name:         "binance-" + marketType,
		marketType:   marketType,
		bus:          bus,
		dialer:       websocket.DefaultDialer,
		minBackoff:   3 * time.Second,
		maxBackoff:   60 * time.Second,
		readTimeout:  60 * time.Second,
		pingInterval: 20 * time.Second,
		writeTimeout: 10 * time.Second,
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

	backoff := c.minBackoff
	for {
		if err := c.runConnection(ctx, streamURL); err != nil {
			if ctx.Err() != nil {
				c.setConnected(false)
				return ctx.Err()
			}
			c.recordError(err)
		}
		c.setConnected(false)
		c.incrementReconnects()
		if err := sleepContext(ctx, backoff); err != nil {
			return err
		}
		backoff *= 2
		if backoff > c.maxBackoff {
			backoff = c.maxBackoff
		}
	}
}

func (c *BinanceWSCollector) runConnection(ctx context.Context, streamURL string) error {
	conn, _, err := c.dialer.DialContext(ctx, streamURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	readTimeout, pingInterval, writeTimeout := c.heartbeatConfig()
	if readTimeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			return err
		}
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(readTimeout))
		})
	}
	c.recordConnected()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	stopHeartbeat := make(chan struct{})
	if pingInterval > 0 {
		go c.keepAlive(ctx, conn, stopHeartbeat, pingInterval, writeTimeout)
	}
	defer close(stopHeartbeat)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
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
		c.recordEvent()
		if readTimeout > 0 {
			if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				return err
			}
		}
	}
}

func (c *BinanceWSCollector) Subscribe(symbols []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.symbols = append([]string(nil), symbols...)
	return nil
}

func (c *BinanceWSCollector) SetReconnectBackoff(min, max time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if min > 0 {
		c.minBackoff = min
	}
	if max > 0 {
		c.maxBackoff = max
	}
	if c.maxBackoff < c.minBackoff {
		c.maxBackoff = c.minBackoff
	}
}

func (c *BinanceWSCollector) SetHeartbeat(readTimeout, pingInterval, writeTimeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if readTimeout > 0 {
		c.readTimeout = readTimeout
	}
	c.pingInterval = pingInterval
	if writeTimeout > 0 {
		c.writeTimeout = writeTimeout
	}
}

func (c *BinanceWSCollector) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Status{
		Name:          c.name,
		Connected:     c.connected,
		Reconnects:    c.reconnects,
		LastEventAt:   c.lastEventAt,
		LastError:     c.lastError,
		Subscribed:    append([]string(nil), c.symbols...),
		LastConnectAt: c.lastConnectAt,
	}
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
	c.mu.RLock()
	symbols := append([]string(nil), c.symbols...)
	c.mu.RUnlock()

	streams := make([]string, 0, len(symbols)*2)
	for _, symbol := range symbols {
		lower := strings.ToLower(symbol)
		streams = append(streams, lower+"@aggTrade")
		if c.marketType == MarketTypeFutures {
			streams = append(streams, lower+"@forceOrder")
		}
	}
	return streams
}

func (c *BinanceWSCollector) recordConnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	c.lastError = ""
	c.lastConnectAt = time.Now().UTC()
}

func (c *BinanceWSCollector) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

func (c *BinanceWSCollector) recordError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err.Error()
}

func (c *BinanceWSCollector) incrementReconnects() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconnects++
}

func (c *BinanceWSCollector) recordEvent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastEventAt = time.Now().UTC()
}

func (c *BinanceWSCollector) heartbeatConfig() (time.Duration, time.Duration, time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.readTimeout, c.pingInterval, c.writeTimeout
}

func (c *BinanceWSCollector) keepAlive(
	ctx context.Context,
	conn *websocket.Conn,
	stop <-chan struct{},
	pingInterval time.Duration,
	writeTimeout time.Duration,
) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			deadline := time.Now().Add(writeTimeout)
			if writeTimeout <= 0 {
				deadline = time.Time{}
			}
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), deadline); err != nil {
				_ = conn.Close()
				return
			}
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
