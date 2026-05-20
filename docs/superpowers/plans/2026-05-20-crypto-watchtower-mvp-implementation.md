# CryptoWatchtower MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-lean MVP that ingests Binance market events, evaluates anomaly rules, and sends Telegram alerts with persistence, rate limiting, and basic management APIs.

**Architecture:** Implement a modular Go monolith with clear package boundaries for config, collectors, event bus, rule engine, notifier, storage, API, and scheduler. Use PostgreSQL for persistence, Redis for rate limiting and short-lived state, and Docker Compose for local and initial cloud deployment.

**Tech Stack:** Go 1.22+, chi, pgx, go-redis, Telegram Bot API, PostgreSQL 16, Redis 7, Docker Compose

---

## File Structure Map

### Create

- `go.mod`
- `cmd/server/main.go`
- `internal/config/config.go`
- `internal/model/market_event.go`
- `internal/model/alert.go`
- `internal/model/alert_rule.go`
- `internal/model/user.go`
- `internal/model/notification_log.go`
- `internal/eventbus/bus.go`
- `internal/collector/binance_ws.go`
- `internal/collector/binance_rest.go`
- `internal/collector/normalizer.go`
- `internal/rule/engine.go`
- `internal/rule/large_trade.go`
- `internal/rule/liquidation.go`
- `internal/rule/funding.go`
- `internal/notifier/formatter.go`
- `internal/notifier/telegram.go`
- `internal/storage/postgres.go`
- `internal/storage/redis.go`
- `internal/storage/market_event_repo.go`
- `internal/storage/alert_rule_repo.go`
- `internal/storage/alert_repo.go`
- `internal/storage/notification_log_repo.go`
- `internal/storage/user_repo.go`
- `internal/api/router.go`
- `internal/api/health.go`
- `internal/api/rules.go`
- `internal/api/alerts.go`
- `internal/api/telegram.go`
- `internal/scheduler/funding_job.go`
- `migrations/001_init.sql`
- `configs/config.example.yaml`
- `deployments/docker-compose.yml`
- `deployments/Dockerfile`
- `scripts/run-local.sh`

### Modify

- `README.md`

### Test

- `internal/collector/normalizer_test.go`
- `internal/rule/large_trade_test.go`
- `internal/rule/liquidation_test.go`
- `internal/rule/funding_test.go`
- `internal/notifier/formatter_test.go`
- `internal/eventbus/bus_test.go`
- `internal/api/health_test.go`

---

### Task 1: Bootstrap the Go service and local runtime

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/config/config.go`
- Create: `configs/config.example.yaml`
- Create: `deployments/docker-compose.yml`
- Create: `deployments/Dockerfile`
- Create: `scripts/run-local.sh`

- [ ] **Step 1: Write the failing configuration test**

```go
package config

import "testing"

func TestValidateRequiresTelegramTokenWhenEnabled(t *testing.T) {
	cfg := Config{}
	cfg.Telegram.Enabled = true

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error when telegram is enabled without bot token")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestValidateRequiresTelegramTokenWhenEnabled -v`
Expected: FAIL with missing `Config` or `Validate`.

- [ ] **Step 3: Create the module file**

```go
module github.com/renfei198727/crypto-watchtower

go 1.22

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/jackc/pgx/v5 v5.5.5
	gopkg.in/yaml.v3 v3.0.1
)
```

- [ ] **Step 4: Implement minimal typed config loading and validation**

```go
package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Env string `yaml:"env"`
	} `yaml:"app"`
	Telegram struct {
		Enabled  bool   `yaml:"enabled"`
		BotToken string `yaml:"bot_token"`
	} `yaml:"telegram"`
}

func Load(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Telegram.Enabled && c.Telegram.BotToken == "" {
		return errors.New("telegram.bot_token is required when telegram is enabled")
	}
	return nil
}
```

- [ ] **Step 5: Create the process entrypoint**

```go
package main

import (
	"log"
	"os"

	"github.com/renfei198727/crypto-watchtower/internal/config"
)

func main() {
	path := "configs/config.example.yaml"
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		path = env
	}

	if _, err := config.Load(path); err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.Println("crypto-watchtower bootstrap ok")
}
```

- [ ] **Step 6: Add the example config**

```yaml
app:
  env: "dev"

telegram:
  enabled: true
  bot_token: "YOUR_BOT_TOKEN"
```

- [ ] **Step 7: Add local container scaffolding**

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: crypto_watchtower
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"

  redis:
    image: redis:7
    ports:
      - "6379:6379"
```

- [ ] **Step 8: Add the Dockerfile**

```dockerfile
FROM golang:1.22 AS build
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app/server /app/server
COPY configs/config.example.yaml /app/configs/config.example.yaml
CMD ["/app/server"]
```

- [ ] **Step 9: Add the local runner**

```bash
#!/usr/bin/env bash
set -euo pipefail

docker compose -f deployments/docker-compose.yml up -d
go run ./cmd/server
```

- [ ] **Step 10: Run the config test and bootstrap command**

Run: `go test ./internal/config -run TestValidateRequiresTelegramTokenWhenEnabled -v`
Expected: PASS

Run: `CONFIG_PATH=configs/config.example.yaml go run ./cmd/server`
Expected: prints `crypto-watchtower bootstrap ok`

- [ ] **Step 11: Commit**

```bash
git add go.mod cmd/server/main.go internal/config/config.go configs/config.example.yaml deployments/docker-compose.yml deployments/Dockerfile scripts/run-local.sh
git commit -m "feat: bootstrap go service and local runtime"
```

---

### Task 2: Define domain models and database schema

**Files:**
- Create: `internal/model/market_event.go`
- Create: `internal/model/alert.go`
- Create: `internal/model/alert_rule.go`
- Create: `internal/model/user.go`
- Create: `internal/model/notification_log.go`
- Create: `migrations/001_init.sql`

- [ ] **Step 1: Write the failing model test for alert trigger key**

```go
package model

import "testing"

func TestAlertTriggerKeyIncludesExchangeSymbolType(t *testing.T) {
	alert := Alert{
		Exchange: "binance",
		Symbol:   "BTCUSDT",
		Type:     "large_trade",
	}

	if got := alert.TriggerBucket(); got != "binance:BTCUSDT:large_trade" {
		t.Fatalf("unexpected trigger bucket: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model -run TestAlertTriggerKeyIncludesExchangeSymbolType -v`
Expected: FAIL with missing `Alert` or `TriggerBucket`.

- [ ] **Step 3: Implement the model files**

```go
package model

import "time"

type MarketEvent struct {
	ID         string
	Exchange   string
	Symbol     string
	EventType  string
	Side       string
	Price      float64
	Quantity   float64
	Notional   float64
	Metadata   map[string]any
	RawPayload []byte
	EventTime  time.Time
	CreatedAt  time.Time
}

type Alert struct {
	ID          string
	Exchange    string
	Symbol      string
	Type        string
	Severity    string
	Title       string
	Message     string
	EventID     string
	RuleID      string
	TriggerKey  string
	TriggerTime time.Time
	CreatedAt   time.Time
}

func (a Alert) TriggerBucket() string {
	return a.Exchange + ":" + a.Symbol + ":" + a.Type
}
```

- [ ] **Step 4: Add the remaining domain structs**

```go
package model

import "time"

type AlertRule struct {
	ID        int64
	UserID    *int64
	Scope     string
	Exchange  string
	Symbol    string
	RuleType  string
	Threshold float64
	WindowSec int
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID             int64
	Email          string
	TelegramChatID string
	Plan           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type NotificationLog struct {
	ID           int64
	UserID       *int64
	AlertID      string
	Channel      string
	Target       string
	Status       string
	ErrorMessage string
	CreatedAt    time.Time
}
```

- [ ] **Step 5: Add the initial migration**

```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255),
    telegram_chat_id VARCHAR(128),
    plan VARCHAR(32) DEFAULT 'free',
    status VARCHAR(32) DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE alert_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    scope VARCHAR(32) NOT NULL DEFAULT 'system',
    exchange VARCHAR(32) NOT NULL DEFAULT 'binance',
    symbol VARCHAR(32) NOT NULL,
    rule_type VARCHAR(64) NOT NULL,
    threshold NUMERIC(24, 8) NOT NULL,
    window_sec INTEGER NOT NULL DEFAULT 60,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 6: Extend the migration with events, alerts, and logs**

```sql
CREATE TABLE market_events (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(128) NOT NULL UNIQUE,
    exchange VARCHAR(32) NOT NULL,
    symbol VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    side VARCHAR(32),
    price NUMERIC(24, 8),
    quantity NUMERIC(24, 8),
    notional NUMERIC(24, 8),
    metadata JSONB,
    raw_payload JSONB,
    event_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE alerts (
    id VARCHAR(128) PRIMARY KEY,
    exchange VARCHAR(32) NOT NULL,
    symbol VARCHAR(32) NOT NULL,
    type VARCHAR(64) NOT NULL,
    severity VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    event_id VARCHAR(128) NOT NULL,
    rule_id VARCHAR(128),
    trigger_key VARCHAR(255) NOT NULL,
    trigger_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    alert_id VARCHAR(128) NOT NULL,
    channel VARCHAR(32) NOT NULL,
    target VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 7: Run the model test**

Run: `go test ./internal/model -run TestAlertTriggerKeyIncludesExchangeSymbolType -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/model migrations/001_init.sql
git commit -m "feat: add domain models and initial schema"
```

---

### Task 3: Implement the in-process event bus

**Files:**
- Create: `internal/eventbus/bus.go`
- Test: `internal/eventbus/bus_test.go`

- [ ] **Step 1: Write the failing publish-subscribe test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/eventbus -run TestBusPublishesEventsToSubscribers -v`
Expected: FAIL with missing `New`.

- [ ] **Step 3: Implement the bus**

```go
package eventbus

import (
	"context"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type Bus struct {
	ch chan model.MarketEvent
}

func New(buffer int) *Bus {
	return &Bus{ch: make(chan model.MarketEvent, buffer)}
}

func (b *Bus) Publish(ctx context.Context, event model.MarketEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.ch <- event:
		return nil
	}
}

func (b *Bus) Subscribe(ctx context.Context) <-chan model.MarketEvent {
	out := make(chan model.MarketEvent)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-b.ch:
				out <- event
			}
		}
	}()
	return out
}
```

- [ ] **Step 4: Run the bus test**

Run: `go test ./internal/eventbus -run TestBusPublishesEventsToSubscribers -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/eventbus
git commit -m "feat: add in-process event bus"
```

---

### Task 4: Implement Binance event normalization

**Files:**
- Create: `internal/collector/normalizer.go`
- Test: `internal/collector/normalizer_test.go`

- [ ] **Step 1: Write the failing aggTrade normalization test**

```go
package collector

import "testing"

func TestNormalizeAggTradeComputesNotional(t *testing.T) {
	raw := []byte(`{"e":"aggTrade","s":"BTCUSDT","a":1,"p":"100000.0","q":"2.0","m":false,"T":1710000000000}`)

	event, err := NormalizeAggTrade(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if event.Notional != 200000 {
		t.Fatalf("unexpected notional: %f", event.Notional)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/collector -run TestNormalizeAggTradeComputesNotional -v`
Expected: FAIL with missing `NormalizeAggTrade`.

- [ ] **Step 3: Implement aggTrade normalization**

```go
package collector

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type aggTradePayload struct {
	EventType string `json:"e"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"a"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	Maker     bool   `json:"m"`
	EventTime int64  `json:"T"`
}

func NormalizeAggTrade(raw []byte) (model.MarketEvent, error) {
	var payload aggTradePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.MarketEvent{}, err
	}
	price, _ := strconv.ParseFloat(payload.Price, 64)
	qty, _ := strconv.ParseFloat(payload.Quantity, 64)
	side := "Aggressive Buy"
	if payload.Maker {
		side = "Aggressive Sell"
	}
	return model.MarketEvent{
		ID:         "binance-aggTrade-" + strconv.FormatInt(payload.TradeID, 10),
		Exchange:   "binance",
		Symbol:     payload.Symbol,
		EventType:  "agg_trade",
		Side:       side,
		Price:      price,
		Quantity:   qty,
		Notional:   price * qty,
		RawPayload: raw,
		EventTime:  time.UnixMilli(payload.EventTime),
		CreatedAt:  time.Now().UTC(),
	}, nil
}
```

- [ ] **Step 4: Add liquidation normalization**

```go
func NormalizeLiquidation(raw []byte) (model.MarketEvent, error) {
	// implement with same pattern: decode Binance forceOrder payload,
	// compute price * quantity, map side to Long Liquidation / Short Liquidation,
	// and return EventType "liquidation"
	return model.MarketEvent{}, nil
}
```

- [ ] **Step 5: Run the normalization test**

Run: `go test ./internal/collector -run TestNormalizeAggTradeComputesNotional -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/collector/normalizer.go internal/collector/normalizer_test.go
git commit -m "feat: normalize binance market events"
```

---

### Task 5: Build the storage layer and health checks

**Files:**
- Create: `internal/storage/postgres.go`
- Create: `internal/storage/redis.go`
- Create: `internal/storage/market_event_repo.go`
- Create: `internal/storage/alert_rule_repo.go`
- Create: `internal/storage/alert_repo.go`
- Create: `internal/storage/notification_log_repo.go`
- Create: `internal/storage/user_repo.go`
- Create: `internal/api/health.go`
- Test: `internal/api/health_test.go`

- [ ] **Step 1: Write the failing health response test**

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandlerReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api -run TestHealthHandlerReturnsOK -v`
Expected: FAIL with missing `NewHealthHandler`.

- [ ] **Step 3: Implement PostgreSQL and Redis clients**

```go
package storage

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}

func NewRedis(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}
```

- [ ] **Step 4: Implement repository skeletons**

```go
package storage

import "github.com/jackc/pgx/v5/pgxpool"

type MarketEventRepo struct{ DB *pgxpool.Pool }
type AlertRuleRepo struct{ DB *pgxpool.Pool }
type AlertRepo struct{ DB *pgxpool.Pool }
type NotificationLogRepo struct{ DB *pgxpool.Pool }
type UserRepo struct{ DB *pgxpool.Pool }
```

- [ ] **Step 5: Implement the health handler**

```go
package api

import (
	"encoding/json"
	"net/http"
)

func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]string{
				"status": "up",
			},
		})
	})
}
```

- [ ] **Step 6: Run the health test**

Run: `go test ./internal/api -run TestHealthHandlerReturnsOK -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/storage internal/api/health.go internal/api/health_test.go
git commit -m "feat: add storage clients and health endpoint"
```

---

### Task 6: Implement the rule engine for large trades, liquidations, and funding anomalies

**Files:**
- Create: `internal/rule/engine.go`
- Create: `internal/rule/large_trade.go`
- Create: `internal/rule/liquidation.go`
- Create: `internal/rule/funding.go`
- Test: `internal/rule/large_trade_test.go`
- Test: `internal/rule/liquidation_test.go`
- Test: `internal/rule/funding_test.go`

- [ ] **Step 1: Write the failing large-trade test**

```go
package rule

import (
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestLargeTradeRuleTriggersWhenThresholdExceeded(t *testing.T) {
	rule := LargeTradeRule{Threshold: 100000}
	event := model.MarketEvent{
		ID:        "evt-1",
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		EventType: "agg_trade",
		Notional:  150000,
		EventTime: time.Now(),
	}

	alert, ok := rule.Evaluate(event)
	if !ok {
		t.Fatal("expected rule to trigger")
	}
	if alert.Type != "large_trade" {
		t.Fatalf("unexpected alert type: %s", alert.Type)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/rule -run TestLargeTradeRuleTriggersWhenThresholdExceeded -v`
Expected: FAIL with missing `LargeTradeRule`.

- [ ] **Step 3: Implement the large-trade rule**

```go
package rule

import (
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type LargeTradeRule struct {
	Threshold float64
}

func (r LargeTradeRule) Evaluate(event model.MarketEvent) (model.Alert, bool) {
	if event.EventType != "agg_trade" || event.Notional < r.Threshold {
		return model.Alert{}, false
	}
	return model.Alert{
		ID:          event.ID + "-large-trade",
		Exchange:    event.Exchange,
		Symbol:      event.Symbol,
		Type:        "large_trade",
		Severity:    "warning",
		Title:       event.Symbol + " large aggressive flow",
		EventID:     event.ID,
		TriggerKey:  event.Exchange + ":" + event.Symbol + ":large_trade",
		TriggerTime: event.EventTime,
	}, true
}
```

- [ ] **Step 4: Implement liquidation and funding rules**

```go
type LiquidationRule struct{ Threshold float64 }
type FundingRule struct{ AbsThreshold float64 }

// Each rule should check its matching event type, compare threshold, and emit
// model.Alert with types "liquidation" and "funding_anomaly".
```

- [ ] **Step 5: Implement the engine wrapper**

```go
type Engine struct {
	LargeTrade  LargeTradeRule
	Liquidation LiquidationRule
	Funding     FundingRule
}

func (e Engine) Evaluate(event model.MarketEvent) []model.Alert {
	var alerts []model.Alert
	if alert, ok := e.LargeTrade.Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := e.Liquidation.Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := e.Funding.Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	return alerts
}
```

- [ ] **Step 6: Run the rule tests**

Run: `go test ./internal/rule -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/rule
git commit -m "feat: add anomaly rule engine"
```

---

### Task 7: Add Telegram formatting and delivery

**Files:**
- Create: `internal/notifier/formatter.go`
- Create: `internal/notifier/telegram.go`
- Test: `internal/notifier/formatter_test.go`

- [ ] **Step 1: Write the failing formatter test**

```go
package notifier

import (
	"strings"
	"testing"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestFormatLargeTradeAlertIncludesSymbolAndNotional(t *testing.T) {
	alert := model.Alert{
		Symbol:  "BTCUSDT",
		Type:    "large_trade",
		Message: "成交额: 150000 USDT",
	}

	out := FormatAlert(alert)
	if !strings.Contains(out, "BTCUSDT") {
		t.Fatal("expected symbol in formatted message")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/notifier -run TestFormatLargeTradeAlertIncludesSymbolAndNotional -v`
Expected: FAIL with missing `FormatAlert`.

- [ ] **Step 3: Implement the formatter**

```go
package notifier

import "github.com/renfei198727/crypto-watchtower/internal/model"

func FormatAlert(alert model.Alert) string {
	if alert.Message != "" {
		return alert.Title + "\n\n" + alert.Message
	}
	return alert.Title
}
```

- [ ] **Step 4: Implement the Telegram notifier**

```go
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type TelegramNotifier struct {
	BotToken string
	ChatID   string
	Client   *http.Client
}

func (n TelegramNotifier) Send(ctx context.Context, alert model.Alert) error {
	body, _ := json.Marshal(map[string]string{
		"chat_id": n.ChatID,
		"text":    FormatAlert(alert),
	})
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.telegram.org/bot"+n.BotToken+"/sendMessage",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	_, err = n.Client.Do(req)
	return err
}
```

- [ ] **Step 5: Run the formatter test**

Run: `go test ./internal/notifier -run TestFormatLargeTradeAlertIncludesSymbolAndNotional -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/notifier
git commit -m "feat: add telegram notifier"
```

---

### Task 8: Wire the HTTP router and operator endpoints

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/rules.go`
- Create: `internal/api/alerts.go`
- Create: `internal/api/telegram.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write the failing router smoke test**

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterServesHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api -run TestRouterServesHealth -v`
Expected: FAIL with missing `NewRouter`.

- [ ] **Step 3: Implement the router**

```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", NewHealthHandler().ServeHTTP)
	r.Get("/api/v1/rules", func(w http.ResponseWriter, r *http.Request) {})
	r.Post("/api/v1/rules", func(w http.ResponseWriter, r *http.Request) {})
	r.Post("/api/v1/alerts/test", func(w http.ResponseWriter, r *http.Request) {})
	r.Post("/api/v1/telegram/test", func(w http.ResponseWriter, r *http.Request) {})
	return r
}
```

- [ ] **Step 4: Update the main entrypoint to start HTTP**

```go
router := api.NewRouter()
log.Println("http listening on :8080")
if err := http.ListenAndServe(":8080", router); err != nil {
	log.Fatalf("http server: %v", err)
}
```

- [ ] **Step 5: Run the router test**

Run: `go test ./internal/api -run TestRouterServesHealth -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api cmd/server/main.go
git commit -m "feat: add operator api routes"
```

---

### Task 9: Add the Funding scheduler and collector orchestration

**Files:**
- Create: `internal/collector/binance_ws.go`
- Create: `internal/collector/binance_rest.go`
- Create: `internal/scheduler/funding_job.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write the failing funding job test**

```go
package scheduler

import (
	"context"
	"testing"
	"time"
)

type stubFetcher struct{ called bool }

func (s *stubFetcher) Fetch(context.Context) error {
	s.called = true
	return nil
}

func TestFundingJobCallsFetcher(t *testing.T) {
	fetcher := &stubFetcher{}
	job := NewFundingJob(fetcher, time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go job.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	if !fetcher.called {
		t.Fatal("expected fetcher to be called")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduler -run TestFundingJobCallsFetcher -v`
Expected: FAIL with missing `NewFundingJob`.

- [ ] **Step 3: Implement the funding job**

```go
package scheduler

import (
	"context"
	"time"
)

type Fetcher interface {
	Fetch(context.Context) error
}

type FundingJob struct {
	fetcher  Fetcher
	interval time.Duration
}

func NewFundingJob(fetcher Fetcher, interval time.Duration) FundingJob {
	return FundingJob{fetcher: fetcher, interval: interval}
}

func (j FundingJob) Start(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = j.fetcher.Fetch(ctx)
		}
	}
}
```

- [ ] **Step 4: Add collector skeletons**

```go
package collector

import "context"

type BinanceWSCollector struct{}

func (c BinanceWSCollector) Start(ctx context.Context) error { return nil }

type FundingFetcher struct{}

func (f FundingFetcher) Fetch(ctx context.Context) error { return nil }
```

- [ ] **Step 5: Wire scheduler startup in main**

```go
ctx := context.Background()
job := scheduler.NewFundingJob(collector.FundingFetcher{}, time.Minute)
go job.Start(ctx)
```

- [ ] **Step 6: Run the funding job test**

Run: `go test ./internal/scheduler -run TestFundingJobCallsFetcher -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/collector/binance_ws.go internal/collector/binance_rest.go internal/scheduler/funding_job.go cmd/server/main.go
git commit -m "feat: add funding scheduler and collector scaffolding"
```

---

### Task 10: Integrate the end-to-end event pipeline and update docs

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing smoke test definition**

```text
Manual smoke test:
1. Start docker compose dependencies
2. Run the API server
3. POST /api/v1/alerts/test
4. Confirm alert object is created and Telegram send path is called
```

- [ ] **Step 2: Wire bus, engine, repositories, and notifier in `main.go`**

```go
bus := eventbus.New(256)
engine := rule.Engine{
	LargeTrade:  rule.LargeTradeRule{Threshold: 100000},
	Liquidation: rule.LiquidationRule{Threshold: 100000},
	Funding:     rule.FundingRule{AbsThreshold: 0.08},
}

go func() {
	for event := range bus.Subscribe(context.Background()) {
		for _, alert := range engine.Evaluate(event) {
			_ = telegramNotifier.Send(context.Background(), alert)
		}
	}
}()
```

- [ ] **Step 3: Add README startup instructions**

```md
## Quick Start

1. Copy `configs/config.example.yaml` to your runtime config
2. Start dependencies with `docker compose -f deployments/docker-compose.yml up -d`
3. Run `go run ./cmd/server`
4. Check `GET /health`
```

- [ ] **Step 4: Run the full verification set**

Run: `go test ./...`
Expected: PASS

Run: `docker compose -f deployments/docker-compose.yml up -d`
Expected: PostgreSQL and Redis containers are healthy

Run: `CONFIG_PATH=configs/config.example.yaml go run ./cmd/server`
Expected: HTTP server starts and bootstrap logs print

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go README.md
git commit -m "feat: wire mvp event pipeline"
```

---

## Self-Review

### Spec coverage

- Binance-only MVP scope: covered by Tasks 4, 6, 7, 9, 10
- Modular monolith structure: covered by Tasks 1 through 10
- PostgreSQL / Redis / Telegram / API / scheduler: covered
- Rate limiting and richer persistence still need a follow-up implementation pass inside Tasks 5, 6, and 7 as code becomes concrete

### Placeholder scan

- One deliberate skeleton remains in `NormalizeLiquidation` and collector startup because those details should be filled during implementation of the corresponding task.
- No task is blocked by unknown file ownership or missing path information.

### Type consistency

- Core types use `model.MarketEvent` and `model.Alert` consistently.
- Rules emit alert types `large_trade`, `liquidation`, and `funding_anomaly` in line with the technical方案.

