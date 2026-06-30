# Discord Webhook Notifier Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Phase 3 Discord/Webhook notification channel without changing the alert evaluation or persistence model.

**Architecture:** Reuse the existing `rule.Sender` boundary and `notification_logs` table. Add a webhook sender in `internal/notifier`, compose enabled senders in `cmd/server`, and update the pipeline so each channel writes its own notification log with channel, target, status, and error message.

**Tech Stack:** Go 1.24, `net/http`, existing `rule.Pipeline`, existing `model.NotificationLog`, PostgreSQL notification logs, Docker Compose.

---

## File Structure

- Create: `internal/notifier/webhook.go` - generic JSON webhook sender for Discord-compatible webhooks.
- Create: `internal/notifier/webhook_test.go` - verifies payload shape and failure handling.
- Modify: `internal/config/config.go` - add `webhook.enabled`, `webhook.url`, `webhook.channel`, `webhook.timeout_sec`, and env overrides.
- Modify: `configs/config.example.yaml` - document disabled-by-default webhook config.
- Modify: `cmd/server/main.go` - compose Telegram and webhook senders based on config.
- Modify: `internal/rule/engine.go` - allow the pipeline to log per-channel send results.
- Modify: `README.md` - document webhook config and verification commands.

## Task 1: Webhook Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `configs/config.example.yaml`

- [x] **Step 1: Write failing config test**

Add this test to `internal/config/config_test.go`:

```go
// TestLoadAppliesWebhookEnvOverrides verifies Discord/Webhook runtime settings can come from env.
//
// Author: monsterfei
// Date: 2026-06-29
func TestLoadAppliesWebhookEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("" +
		"binance:\n" +
		"  enabled: false\n" +
		"okx:\n" +
		"  enabled: true\n" +
		"  public_ws_base_url: wss://example/ws\n" +
		"  rest_base_url: https://example.test\n" +
		"  symbols: [BTCUSDT]\n" +
		"postgres:\n" +
		"  dsn: postgres://from-file\n" +
		"redis:\n" +
		"  addr: localhost:6379\n" +
		"telegram:\n" +
		"  enabled: false\n" +
		"api:\n" +
		"  bearer_token: file-token\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CW_WEBHOOK_ENABLED", "true")
	t.Setenv("CW_WEBHOOK_URL", "https://discord.example/webhook")
	t.Setenv("CW_WEBHOOK_CHANNEL", "discord")
	t.Setenv("CW_WEBHOOK_TIMEOUT_SEC", "7")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Webhook.Enabled || cfg.Webhook.URL != "https://discord.example/webhook" || cfg.Webhook.Channel != "discord" || cfg.Webhook.TimeoutSec != 7 {
		t.Fatalf("unexpected webhook config: %+v", cfg.Webhook)
	}
}
```

- [x] **Step 2: Run failing config test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/config -run Webhook -v'
```

Expected: FAIL because `Config.Webhook` does not exist.

- [x] **Step 3: Implement config**

Add `WebhookConfig` to `internal/config/config.go`:

```go
// WebhookConfig contains optional generic webhook notification settings.
//
// Author: monsterfei
// Date: 2026-06-29
type WebhookConfig struct {
	Enabled    bool   `yaml:"enabled"`
	URL        string `yaml:"url"`
	Channel    string `yaml:"channel"`
	TimeoutSec int    `yaml:"timeout_sec"`
}
```

Add `Webhook WebhookConfig` to `Config`, env overrides for `CW_WEBHOOK_ENABLED`, `CW_WEBHOOK_URL`, `CW_WEBHOOK_CHANNEL`, `CW_WEBHOOK_TIMEOUT_SEC`, and validation that `webhook.url` is required only when enabled.

- [x] **Step 4: Run config test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/config -run Webhook -v'
```

Expected: PASS.

## Task 2: Webhook Sender

**Files:**
- Create: `internal/notifier/webhook.go`
- Create: `internal/notifier/webhook_test.go`

- [x] **Step 1: Write failing sender tests**

Create `internal/notifier/webhook_test.go` with tests that:

```go
func TestWebhookNotifierSendsFormattedAlert(t *testing.T)
func TestWebhookNotifierReturnsErrorOnNon2xx(t *testing.T)
```

The first test should use `httptest.NewServer`, call `WebhookNotifier.Send`, and assert JSON contains `content` with the alert title and message.

- [x] **Step 2: Run failing sender tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/notifier -run Webhook -v'
```

Expected: FAIL because `WebhookNotifier` does not exist.

- [x] **Step 3: Implement sender**

Create `internal/notifier/webhook.go` with:

```go
type WebhookNotifier struct {
	URL     string
	Channel string
	Client  *http.Client
}

func NewWebhookNotifier(url string, channel string, client *http.Client) WebhookNotifier
func (n WebhookNotifier) Send(ctx context.Context, alert model.Alert) error
```

Send JSON payload:

```json
{"content":"<FormatAlert output>"}
```

Treat any non-2xx status as an error.

- [x] **Step 4: Run sender tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/notifier -run Webhook -v'
```

Expected: PASS.

## Task 3: Multi-Channel Pipeline Logging

**Files:**
- Modify: `internal/rule/engine.go`
- Modify: `internal/rule/engine_test.go`

- [x] **Step 1: Write failing pipeline test**

Add a test proving one alert sent through two channels creates two notification logs:

```go
func TestPipelineLogsEachNotificationChannel(t *testing.T)
```

Use fake senders named `telegram` and `discord`, then assert two log rows are inserted with distinct `channel` values.

- [x] **Step 2: Run failing pipeline test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/rule -run PipelineLogsEachNotificationChannel -v'
```

Expected: FAIL because the current pipeline logs only `telegram/default`.

- [x] **Step 3: Implement channel-aware sender composition**

Introduce a small sender result surface:

```go
type NamedSender interface {
	Name() string
	Target() string
	Send(context.Context, model.Alert) error
}
```

Adapt Telegram and webhook senders through wrappers rather than changing their public behavior. The pipeline should insert one `notification_logs` row per enabled sender.

- [x] **Step 4: Run pipeline tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/rule -v'
```

Expected: PASS.

## Task 4: Runtime Wiring And Docs

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `README.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`

- [x] **Step 1: Wire webhook sender**

In `cmd/server/main.go`, build enabled senders:

```go
senders := []rule.NamedSender{}
if cfg.Telegram.Enabled {
	senders = append(senders, rule.NewNamedSender("telegram", "default", tg))
}
if cfg.Webhook.Enabled {
	senders = append(senders, rule.NewNamedSender(cfg.Webhook.Channel, cfg.Webhook.URL, notifier.NewWebhookNotifier(cfg.Webhook.URL, cfg.Webhook.Channel, nil)))
}
```

Expected: when webhook is disabled, current Telegram behavior remains unchanged.

- [x] **Step 2: Run integration verification**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
```

Expected: PASS.

- [x] **Step 3: Update docs**

Document:

```text
webhook.enabled
webhook.url
webhook.channel
webhook.timeout_sec
CW_WEBHOOK_ENABLED
CW_WEBHOOK_URL
CW_WEBHOOK_CHANNEL
CW_WEBHOOK_TIMEOUT_SEC
```

Expected: README explains that Discord/Webhook is optional and disabled by default.

## Verification Gate

- [x] `docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'`
- [x] `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh`
- [x] `git diff --check`

## Execution Notes

- 2026-06-29: Webhook config, sender, multi-channel notification logging, runtime wiring, README/config docs, Go tests, Docker Compose smoke, and diff check completed.
