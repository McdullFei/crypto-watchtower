# OKX Exchange Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add OKX as the second exchange so CryptoWatchtower can ingest OKX spot/swap trades, swap liquidation orders, and swap funding rates into the existing alert pipeline.

**Architecture:** Keep the existing Go modular monolith and reuse `model.MarketEvent`, the event bus, rule engine, notifier, storage, health checks, and Admin UI. Add OKX-specific config, instrument metadata, normalizers, and a public WebSocket collector; do not introduce a new persistence model unless verification proves the existing `exchange` fields are insufficient.

**Tech Stack:** Go 1.24, `gorilla/websocket`, `net/http`, PostgreSQL 16.14, Redis 7.0.15, Docker Compose, OKX API v5 public WebSocket/REST.
monsterfei
---

## Active Execution Checklist

- [x] Plan created under `docs/superpowers/plans`.
- [x] Baseline tests checked before implementation.
- [x] OKX config added and validated.
- [x] OKX symbol/instrument metadata support added.
- [x] OKX normalizers implemented with sample-payload tests.
- [x] OKX WebSocket collector implemented with subscription and parse tests.
- [x] Runtime wiring starts OKX collectors when enabled.
- [x] Admin/API exchange filtering added.
- [x] README and project plans synchronized.
- [x] Final verification run and plan checkboxes updated.

## Official OKX Interfaces

- Public WebSocket URL: `wss://ws.okx.com:8443/ws/v5/public`
- Public REST base URL: `https://www.okx.com`
- Spot trades channel: `{"channel":"trades","instId":"BTC-USDT"}`
- Swap trades channel: `{"channel":"trades","instId":"BTC-USDT-SWAP"}`
- Swap liquidation channel: `{"channel":"liquidation-orders","instType":"SWAP"}`
- Swap funding channel: `{"channel":"funding-rate","instId":"BTC-USDT-SWAP"}`
- Instrument metadata endpoint: `GET /api/v5/public/instruments?instType=SPOT` and `GET /api/v5/public/instruments?instType=SWAP`

Important implementation notes:

- OKX `trades` and `books-l2-tbt` may have live-environment trading-tier restrictions. If a live subscription returns `64003`, record it as a provider limitation instead of treating it as a parser bug.
- OKX derivative `sz` means contract count. For USDT linear swaps, notional should use `price * size * ctVal`; do not calculate swap notional as `price * size` unless the instrument metadata proves `ctVal=1`.
- Keep first-slice support limited to USDT spot and USDT linear swap symbols matching current Binance symbols.

## File Structure Map

### Create

- `internal/collector/okx_instrument.go` - OKX instrument metadata model, symbol mapping, REST fetcher, and notional helpers.
- `internal/collector/okx_normalizer.go` - OKX trade, liquidation, and funding payload normalization to `model.MarketEvent`.
- `internal/collector/okx_ws.go` - OKX public WebSocket collector with subscribe, heartbeat, reconnect, parse, and status behavior.
- `internal/collector/okx_instrument_test.go` - instrument metadata and notional unit tests.
- `internal/collector/okx_normalizer_test.go` - OKX sample payload normalization tests.
- `internal/collector/okx_ws_test.go` - OKX collector subscription and parse tests.

### Modify

- `configs/config.example.yaml` - add disabled-by-default OKX config.
- `internal/config/config.go` - load, default, env override, and validate OKX config.
- `internal/config/config_test.go` - config tests for OKX enabled/disabled validation.
- `cmd/server/main.go` - start OKX collectors/funding when enabled and expose collector health.
- `internal/storage/list_filter.go` - add exchange filter.
- `internal/storage/alert_repo.go` - filter alert lists by exchange.
- `internal/storage/alert_rule_repo.go` - filter rule lists by exchange.
- `internal/storage/market_event_repo.go` - filter event lists by exchange.
- `internal/api/admin.go` - parse `exchange` from Admin API requests.
- `internal/admin/service.go` - map API filters to storage filters with exchange.
- `internal/api/adminui/app.js` and `internal/api/adminui/index.html` - add exchange filter controls.
- `internal/api/admin_test.go` - cover Admin exchange filtering.
- `README.md` - document OKX scope, config, and verification.
- `docs/plan/币圈异动监控平台总体开发计划.md` - mark OKX slice progress.
- `docs/superpowers/plans/[1]2026-05-20-crypto-watchtower-mvp-implementation.md` - keep historical plan status synchronized.

## Task 1: Baseline And OKX Config

**Files:**
- Modify: `configs/config.example.yaml`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [x] **Step 1: Run baseline tests**

Run:

```bash
go test ./...
```

Expected: PASS before feature implementation, or record the exact pre-existing failure before continuing.

- [x] **Step 2: Write failing OKX config tests**

Add tests that prove `okx.enabled=false` does not require OKX URLs, and `okx.enabled=true` requires `public_ws_base_url`, `rest_base_url`, and at least one symbol.

- [x] **Step 3: Run focused config test and verify RED**

Run:

```bash
go test ./internal/config -run OKX -v
```

Expected: FAIL because the OKX config fields and validation do not exist yet.

- [x] **Step 4: Implement OKX config**

Add:

```yaml
okx:
  enabled: false
  public_ws_base_url: "wss://ws.okx.com:8443/ws/v5/public"
  rest_base_url: "https://www.okx.com"
  symbols:
    - BTCUSDT
    - ETHUSDT
    - SOLUSDT
```

Add matching Go config fields and environment overrides:

- `CW_OKX_ENABLED`
- `CW_OKX_PUBLIC_WS_BASE_URL`
- `CW_OKX_REST_BASE_URL`
- `CW_OKX_SYMBOLS`

- [x] **Step 5: Run config tests and mark task complete**

Run:

```bash
go test ./internal/config -v
```

Expected: PASS.

## Task 2: OKX Instrument Metadata

**Files:**
- Create: `internal/collector/okx_instrument.go`
- Create: `internal/collector/okx_instrument_test.go`

- [x] **Step 1: Write failing symbol mapping and notional tests**

Cover:

- `BTCUSDT` maps to spot `BTC-USDT`.
- `BTCUSDT` maps to swap `BTC-USDT-SWAP`.
- Spot notional is `price * size`.
- USDT linear swap notional is `price * size * ctVal`.

- [x] **Step 2: Run focused test and verify RED**

Run:

```bash
go test ./internal/collector -run OKXInstrument -v
```

Expected: FAIL because OKX instrument helpers do not exist yet.

- [x] **Step 3: Implement instrument helpers and REST fetcher**

Implement a bounded `OKXInstrumentStore` keyed by native `instId`. The REST fetcher should parse only fields needed in this phase: `instId`, `instType`, `ctVal`, `ctValCcy`, and `settleCcy`.

- [x] **Step 4: Run instrument tests**

Run:

```bash
go test ./internal/collector -run OKXInstrument -v
```

Expected: PASS.

## Task 3: OKX Normalizers

**Files:**
- Create: `internal/collector/okx_normalizer.go`
- Create: `internal/collector/okx_normalizer_test.go`

- [x] **Step 1: Write failing sample-payload tests**

Use OKX official sample shapes for:

- `trades` message with `instId`, `tradeId`, `px`, `sz`, `side`, `ts`, `count`, `source`, `seqId`.
- `liquidation-orders` message with `instId`, `instType`, and `details` containing `bkPx`, `sz`, `side`, `posSide`, `ts`.
- `funding-rate` message with `instId`, `fundingRate`, `fundingTime`, `nextFundingTime`, `settState`, `settFundingRate`, `ts`.

- [x] **Step 2: Run focused normalizer tests and verify RED**

Run:

```bash
go test ./internal/collector -run OKXNormalize -v
```

Expected: FAIL because OKX normalizers do not exist yet.

- [x] **Step 3: Implement normalizers**

Normalize to:

- `Exchange: "okx"`
- `MarketType: "spot"` for `BTC-USDT`, `MarketType: "futures"` for `BTC-USDT-SWAP`
- `EventType: "agg_trade"`, `"liquidation"`, or `"funding"`
- `Side: "Aggressive Buy"` for OKX `side=buy`, `Side: "Aggressive Sell"` for OKX `side=sell`
- Liquidation side should prefer `posSide`: `long -> Long Liquidation`, `short -> Short Liquidation`, otherwise derive from `side`.
- Funding metadata must include at least `funding_rate`, `native_inst_id`, `next_funding_time`, and `sett_state`.

- [x] **Step 4: Run normalizer tests**

Run:

```bash
go test ./internal/collector -run OKXNormalize -v
```

Expected: PASS.

## Task 4: OKX WebSocket Collector

**Files:**
- Create: `internal/collector/okx_ws.go`
- Create: `internal/collector/okx_ws_test.go`

- [x] **Step 1: Write failing collector tests**

Cover:

- Subscribe sends `trades` args for spot and swap instruments.
- Subscribe sends one `liquidation-orders` arg for `instType=SWAP`.
- Subscribe sends `funding-rate` args for swap instruments.
- Error ack records `LastError`.
- Data messages publish normalized events.

- [x] **Step 2: Run focused collector tests and verify RED**

Run:

```bash
go test ./internal/collector -run OKXWS -v
```

Expected: FAIL because OKX collector does not exist yet.

- [x] **Step 3: Implement OKX collector**

Implement OKX collector with the same runtime posture as Binance:

- `Start(context.Context) error`
- `Subscribe([]string) error`
- `Status() Status`
- reconnect with bounded backoff
- read timeout and ping keepalive
- JSON subscribe request after connection opens
- ignore subscribe ack, record error ack, publish normalized data

- [x] **Step 4: Run collector tests**

Run:

```bash
go test ./internal/collector -run OKXWS -v
```

Expected: PASS.

## Task 5: Runtime Wiring

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/api/health_test.go`

- [x] **Step 1: Write failing wiring test or focused startup helper test**

Cover that OKX-enabled config contributes OKX collector status providers without disabling Binance collectors.

- [x] **Step 2: Implement runtime wiring**

When `okx.enabled=true`:

- Fetch OKX spot/swap instruments before starting OKX collector.
- Start OKX WebSocket collector.
- Include OKX collector in `/health`.
- Keep Binance enabled using existing config and behavior.

- [x] **Step 3: Run relevant tests**

Run:

```bash
go test ./cmd/server ./internal/api ./internal/collector -v
```

Expected: PASS.

## Task 6: Admin Exchange Filtering

**Files:**
- Modify: `internal/storage/list_filter.go`
- Modify: `internal/storage/alert_repo.go`
- Modify: `internal/storage/alert_rule_repo.go`
- Modify: `internal/storage/market_event_repo.go`
- Modify: `internal/api/admin.go`
- Modify: `internal/admin/service.go`
- Modify: `internal/api/admin_test.go`
- Modify: `internal/api/adminui/index.html`
- Modify: `internal/api/adminui/app.js`

- [x] **Step 1: Write failing Admin API filter tests**

Cover that `exchange=okx` is parsed and forwarded to the Admin service filter.

- [x] **Step 2: Implement API and storage exchange filtering**

Add `Exchange string` to list filter structs and SQL conditions. Keep default behavior unchanged when `exchange` is empty.

- [x] **Step 3: Add Admin UI exchange filter**

Add a compact exchange input/select alongside existing list filters. Do not add marketing copy or unrelated UI changes.

- [x] **Step 4: Run Admin tests**

Run:

```bash
go test ./internal/api ./internal/admin ./internal/storage -v
```

Expected: PASS.

## Task 7: Documentation And Plan Status

**Files:**
- Modify: `README.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`
- Modify: `docs/superpowers/plans/[1]2026-05-20-crypto-watchtower-mvp-implementation.md`
- Modify: `docs/superpowers/plans/[2]2026-06-29-okx-exchange-integration.md`

- [x] **Step 1: Update docs**

Document:

- OKX supported channels and current scope.
- OKX config keys and environment variables.
- OKX notional calculation constraint for swaps.
- Known OKX `trades` live-tier limitation.
- Verification commands.

- [x] **Step 2: Mark completed checkboxes in this plan**

Every task actually completed during execution must be marked `[x]`.

- [x] **Step 3: Run final verification**

Run:

```bash
go test ./...
```

Expected: PASS.

Run Docker smoke if dependencies are available:

```bash
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

Expected: PASS or record the environmental reason it could not run.
