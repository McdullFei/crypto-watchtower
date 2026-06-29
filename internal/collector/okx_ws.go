package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// OKXWSCollector subscribes to OKX public market-data channels.
//
// Author: monsterfei
// Date: 2026-06-29
type OKXWSCollector struct {
	baseURL       string
	bus           *eventbus.Bus
	instruments   OKXInstrumentStore
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

// NewOKXWSCollector creates an OKX public WebSocket collector.
//
// Author: monsterfei
// Date: 2026-06-29
func NewOKXWSCollector(baseURL string, bus *eventbus.Bus, instruments OKXInstrumentStore) *OKXWSCollector {
	return &OKXWSCollector{
		baseURL:      baseURL,
		bus:          bus,
		instruments:  instruments,
		dialer:       websocket.DefaultDialer,
		minBackoff:   3 * time.Second,
		maxBackoff:   60 * time.Second,
		readTimeout:  60 * time.Second,
		pingInterval: 20 * time.Second,
		writeTimeout: 10 * time.Second,
	}
}

// Name returns the collector name used in health output.
//
// Author: monsterfei
// Date: 2026-06-29
func (c *OKXWSCollector) Name() string {
	return "okx-public"
}

// Start connects to OKX, subscribes to public channels, and publishes events.
//
// Author: monsterfei
// Date: 2026-06-29
func (c *OKXWSCollector) Start(ctx context.Context) error {
	if len(c.symbols) == 0 {
		return errors.New("no symbols subscribed")
	}
	if c.baseURL == "" {
		return errors.New("base URL is required")
	}

	backoff := c.minBackoff
	for {
		if err := c.runConnection(ctx); err != nil {
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

// Subscribe replaces the compact symbols used to build OKX channel args.
//
// Author: monsterfei
// Date: 2026-06-29
func (c *OKXWSCollector) Subscribe(symbols []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.symbols = append([]string(nil), symbols...)
	return nil
}

// SetReconnectBackoff updates reconnect timings for runtime or tests.
//
// Author: monsterfei
// Date: 2026-06-29
func (c *OKXWSCollector) SetReconnectBackoff(min, max time.Duration) {
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

// SetHeartbeat updates read and ping settings for runtime or tests.
//
// Author: monsterfei
// Date: 2026-06-29
func (c *OKXWSCollector) SetHeartbeat(readTimeout, pingInterval, writeTimeout time.Duration) {
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

// Status returns the current OKX collector health state.
//
// Author: monsterfei
// Date: 2026-06-29
func (c *OKXWSCollector) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Status{
		Name:          c.Name(),
		Connected:     c.connected,
		Reconnects:    c.reconnects,
		LastEventAt:   c.lastEventAt,
		LastError:     c.lastError,
		Subscribed:    append([]string(nil), c.symbols...),
		LastConnectAt: c.lastConnectAt,
	}
}

func (c *OKXWSCollector) runConnection(ctx context.Context) error {
	conn, _, err := c.dialer.DialContext(ctx, c.baseURL, nil)
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

	if err := conn.WriteJSON(okxSubscribeRequest{
		ID:        "crypto-watchtower-okx",
		Operation: "subscribe",
		Args:      c.subscribeArgs(),
	}); err != nil {
		return err
	}

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
		events, err := c.handleMessage(payload)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := c.bus.Publish(ctx, event); err != nil {
				return err
			}
			c.recordEvent()
		}
		if readTimeout > 0 {
			if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				return err
			}
		}
	}
}

func (c *OKXWSCollector) handleMessage(payload []byte) ([]model.MarketEvent, error) {
	var envelope okxMessageEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, err
	}
	if envelope.Event == "error" {
		c.recordError(fmt.Errorf("okx websocket error %s: %s", envelope.Code, envelope.Message))
		return nil, nil
	}
	if envelope.Event != "" {
		return nil, nil
	}

	var events []model.MarketEvent
	var err error
	switch envelope.Arg.Channel {
	case "trades":
		events, err = NormalizeOKXTrades(payload, c.instruments)
	case "liquidation-orders":
		events, err = NormalizeOKXLiquidations(payload, c.instruments)
		events = c.filterSubscribedLiquidations(events)
	case "funding-rate":
		events, err = NormalizeOKXFunding(payload)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *OKXWSCollector) subscribeArgs() []okxChannelArg {
	c.mu.RLock()
	symbols := append([]string(nil), c.symbols...)
	c.mu.RUnlock()

	args := make([]okxChannelArg, 0, len(symbols)*3+1)
	for _, symbol := range symbols {
		spot := OKXSpotInstID(symbol)
		swap := OKXSwapInstID(symbol)
		args = append(args,
			okxChannelArg{Channel: "trades", InstID: spot},
			okxChannelArg{Channel: "trades", InstID: swap},
			okxChannelArg{Channel: "funding-rate", InstID: swap},
		)
	}
	args = append(args, okxChannelArg{Channel: "liquidation-orders", InstType: "SWAP"})
	return args
}

func (c *OKXWSCollector) filterSubscribedLiquidations(events []model.MarketEvent) []model.MarketEvent {
	c.mu.RLock()
	allowed := make(map[string]struct{}, len(c.symbols))
	for _, symbol := range c.symbols {
		allowed[OKXSwapInstID(symbol)] = struct{}{}
	}
	c.mu.RUnlock()

	filtered := events[:0]
	for _, event := range events {
		native, _ := event.Metadata["native_inst_id"].(string)
		if _, ok := allowed[native]; ok {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func (c *OKXWSCollector) recordConnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	c.lastError = ""
	c.lastConnectAt = time.Now().UTC()
}

func (c *OKXWSCollector) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

func (c *OKXWSCollector) recordError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err.Error()
}

func (c *OKXWSCollector) incrementReconnects() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconnects++
}

func (c *OKXWSCollector) recordEvent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastEventAt = time.Now().UTC()
}

func (c *OKXWSCollector) heartbeatConfig() (time.Duration, time.Duration, time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.readTimeout, c.pingInterval, c.writeTimeout
}

func (c *OKXWSCollector) keepAlive(
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

type okxSubscribeRequest struct {
	ID        string          `json:"id,omitempty"`
	Operation string          `json:"op"`
	Args      []okxChannelArg `json:"args"`
}

type okxChannelArg struct {
	Channel  string `json:"channel"`
	InstID   string `json:"instId,omitempty"`
	InstType string `json:"instType,omitempty"`
}

type okxMessageEnvelope struct {
	Event   string        `json:"event"`
	Code    string        `json:"code"`
	Message string        `json:"msg"`
	Arg     okxChannelArg `json:"arg"`
}
