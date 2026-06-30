# AI Market Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Phase 3 15-minute AI market summary job that reads persisted alerts, bounded event aggregates, and funding state without affecting the real-time alert pipeline.

**Architecture:** Keep the summary path independent from collectors and `rule.Pipeline`: a scheduler calls a summary service, the service builds a bounded snapshot from PostgreSQL, a generator produces disclaimer-bearing text, and a repository stores each run result. Default config keeps the job disabled so existing local and Docker smoke flows remain unchanged until operators opt in; local smoke can use the deterministic template provider, while production AI can use an OpenAI-compatible HTTP provider.

**Tech Stack:** Go 1.24, PostgreSQL, existing `storage.Repositories`, existing scheduler pattern, `net/http` for optional OpenAI-compatible summary generation, Docker Compose.

---

## Acceptance Criteria

- [x] Summary job is disabled by default and controlled by config/env.
- [x] A 15-minute summary run reads only bounded data from `alerts` and `market_events`.
- [x] Summary text always includes `不构成投资建议`.
- [x] Summary generation or persistence errors are logged and do not stop collectors, HTTP, or real-time alert delivery.
- [x] Summary records are queryable through storage and can be shown later in Admin UI.

## File Structure

- Create: `internal/model/market_summary.go` - persisted summary model.
- Create: `internal/storage/market_summary_repo.go` - insert/list repository for generated summaries.
- Create: `migrations/002_market_summaries.sql` - database table and indexes for summaries.
- Create: `internal/summary/aggregator.go` - bounded snapshot builder over alerts/events.
- Create: `internal/summary/generator.go` - prompt and generator interfaces plus deterministic fallback generator.
- Create: `internal/summary/openai_compatible.go` - optional OpenAI-compatible chat-completions generator.
- Create: `internal/summary/service.go` - run-once orchestration with failure isolation.
- Create: `internal/scheduler/summary_job.go` - interval job wrapper around the summary service.
- Modify: `internal/config/config.go` - add `summary.enabled`, `summary.interval_sec`, `summary.window_sec`, `summary.max_items`, `summary.disclaimer`, `summary.provider`, `summary.api_base_url`, `summary.api_key`, `summary.model`, `summary.timeout_sec`.
- Modify: `internal/storage/list_filter.go` - add optional `Since time.Time` for bounded repository reads.
- Modify: `internal/storage/alert_repo.go` and `internal/storage/market_event_repo.go` - honor `ListFilter.Since`.
- Modify: `internal/storage/repositories.go` - expose `MarketSummaries`.
- Modify: `cmd/server/main.go` - start the summary job only when enabled.
- Modify: `configs/config.example.yaml`, `deployments/docker-compose.yml`, `README.md`, and `docs/plan/币圈异动监控平台总体开发计划.md` - document runtime settings and phase status.

## Task 1: Summary Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `configs/config.example.yaml`

- [x] **Step 1: Write failing config test**

Add this test to `internal/config/config_test.go`:

```go
// TestLoadAppliesSummaryEnvOverrides verifies AI summary runtime settings can come from env.
//
// Author: monsterfei
// Date: 2026-06-29
func TestLoadAppliesSummaryEnvOverrides(t *testing.T) {
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

	t.Setenv("CW_SUMMARY_ENABLED", "true")
	t.Setenv("CW_SUMMARY_INTERVAL_SEC", "900")
	t.Setenv("CW_SUMMARY_WINDOW_SEC", "900")
	t.Setenv("CW_SUMMARY_MAX_ITEMS", "40")
	t.Setenv("CW_SUMMARY_PROVIDER", "template")
	t.Setenv("CW_SUMMARY_DISCLAIMER", "不构成投资建议")
	t.Setenv("CW_SUMMARY_API_BASE_URL", "https://api.example.test/v1")
	t.Setenv("CW_SUMMARY_API_KEY", "summary-key")
	t.Setenv("CW_SUMMARY_MODEL", "summary-model")
	t.Setenv("CW_SUMMARY_TIMEOUT_SEC", "8")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Summary.Enabled || cfg.Summary.IntervalSec != 900 || cfg.Summary.WindowSec != 900 || cfg.Summary.MaxItems != 40 || cfg.Summary.Provider != "template" || cfg.Summary.Disclaimer != "不构成投资建议" || cfg.Summary.APIBaseURL != "https://api.example.test/v1" || cfg.Summary.APIKey != "summary-key" || cfg.Summary.Model != "summary-model" || cfg.Summary.TimeoutSec != 8 {
		t.Fatalf("unexpected summary config: %+v", cfg.Summary)
	}
}
```

- [x] **Step 2: Run failing config test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/config -run Summary -v'
```

Expected: FAIL because `Config.Summary` does not exist.

- [x] **Step 3: Implement config**

Add this type and field:

```go
// SummaryConfig contains optional AI market summary job settings.
//
// Author: monsterfei
// Date: 2026-06-29
type SummaryConfig struct {
	Enabled     bool   `yaml:"enabled"`
	IntervalSec int    `yaml:"interval_sec"`
	WindowSec   int    `yaml:"window_sec"`
	MaxItems    int    `yaml:"max_items"`
	Provider    string `yaml:"provider"`
	Disclaimer  string `yaml:"disclaimer"`
	APIBaseURL  string `yaml:"api_base_url"`
	APIKey      string `yaml:"api_key"`
	Model       string `yaml:"model"`
	TimeoutSec  int    `yaml:"timeout_sec"`
}
```

Add `Summary SummaryConfig` to `Config`, env overrides for `CW_SUMMARY_ENABLED`, `CW_SUMMARY_INTERVAL_SEC`, `CW_SUMMARY_WINDOW_SEC`, `CW_SUMMARY_MAX_ITEMS`, `CW_SUMMARY_PROVIDER`, `CW_SUMMARY_DISCLAIMER`, `CW_SUMMARY_API_BASE_URL`, `CW_SUMMARY_API_KEY`, `CW_SUMMARY_MODEL`, and `CW_SUMMARY_TIMEOUT_SEC`, plus defaults:

```go
cfg.Summary.IntervalSec = 900
cfg.Summary.WindowSec = 900
cfg.Summary.MaxItems = 50
cfg.Summary.Provider = "template"
cfg.Summary.Disclaimer = "不构成投资建议"
cfg.Summary.TimeoutSec = 15
```

Validation rule: when enabled, interval, window, max items, provider, and disclaimer must all be non-empty/non-zero. If provider is `openai_compatible`, `api_base_url`, `api_key`, `model`, and `timeout_sec` are also required.

- [x] **Step 4: Update example config**

Add to `configs/config.example.yaml`:

```yaml
summary:
  enabled: false
  interval_sec: 900
  window_sec: 900
  max_items: 50
  provider: "template"
  disclaimer: "不构成投资建议"
  api_base_url: ""
  api_key: ""
  model: ""
  timeout_sec: 15
```

- [x] **Step 5: Run config test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/config -run Summary -v'
```

Expected: PASS.

## Task 2: Summary Storage

**Files:**
- Create: `internal/model/market_summary.go`
- Create: `internal/storage/market_summary_repo.go`
- Create: `migrations/002_market_summaries.sql`
- Modify: `internal/storage/repositories.go`
- Modify: `internal/integration/postgres_redis_test.go`

- [x] **Step 1: Write failing integration coverage**

Extend `internal/integration/postgres_redis_test.go` after notification log assertions:

```go
	summaryID := "summary-" + suffix
	if err := repos.MarketSummaries.Insert(ctx, model.MarketSummary{
		ID:         summaryID,
		WindowFrom: now.Add(-15 * time.Minute),
		WindowTo:   now,
		Provider:   "template",
		Status:     "generated",
		Content:    "BTCUSDT 异动摘要。不构成投资建议",
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("insert market summary: %v", err)
	}
	summaries, err := repos.MarketSummaries.List(ctx, storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list market summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != summaryID || summaries[0].Status != "generated" {
		t.Fatalf("unexpected market summaries: %+v", summaries)
	}
```

- [x] **Step 2: Run failing integration test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/integration -tags integration -run PostgresRedis -v'
```

Expected: FAIL because `MarketSummary` and `MarketSummaries` do not exist.

- [x] **Step 3: Add migration**

Create `migrations/002_market_summaries.sql`:

```sql
CREATE TABLE IF NOT EXISTS market_summaries (
    id VARCHAR(128) PRIMARY KEY,
    window_from TIMESTAMP NOT NULL,
    window_to TIMESTAMP NOT NULL,
    provider VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_summaries_window
    ON market_summaries(window_to DESC);
```

- [x] **Step 4: Add model and repository**

Create `internal/model/market_summary.go`:

```go
package model

import "time"

// MarketSummary stores one generated market summary window.
//
// Author: monsterfei
// Date: 2026-06-29
type MarketSummary struct {
	ID           string
	WindowFrom   time.Time
	WindowTo     time.Time
	Provider     string
	Status       string
	Content      string
	ErrorMessage string
	CreatedAt    time.Time
}
```

Create repository methods:

```go
func (r MarketSummaryRepo) Insert(ctx context.Context, summary model.MarketSummary) error
func (r MarketSummaryRepo) List(ctx context.Context, filter ListFilter) ([]model.MarketSummary, error)
```

`List` must order by `window_to DESC` and use `normalizedLimit(filter.Limit)`.

- [x] **Step 5: Expose repository set**

Add `MarketSummaries MarketSummaryRepo` to `storage.Repositories` and initialize it in `NewRepositories`.

- [x] **Step 6: Run integration test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/integration -tags integration -run PostgresRedis -v'
```

Expected: PASS when PostgreSQL and Redis integration dependencies are available.

## Task 3: Bounded Summary Aggregation

**Files:**
- Create: `internal/summary/aggregator.go`
- Create: `internal/summary/aggregator_test.go`
- Modify: `internal/storage/list_filter.go`
- Modify: `internal/storage/alert_repo.go`
- Modify: `internal/storage/market_event_repo.go`

- [x] **Step 1: Write failing aggregation test**

Create `internal/summary/aggregator_test.go`:

```go
func TestAggregatorBuildsBoundedSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	alerts := []model.Alert{
		{ID: "a1", Exchange: "binance", Symbol: "BTCUSDT", Type: "large_trade", Title: "BTC large trade", CreatedAt: now.Add(-time.Minute)},
		{ID: "a2", Exchange: "okx", Symbol: "ETHUSDT", Type: "funding_anomaly", Title: "ETH funding", CreatedAt: now.Add(-2 * time.Minute)},
	}
	events := []model.MarketEvent{
		{ID: "e1", Exchange: "binance", Symbol: "BTCUSDT", EventType: "agg_trade", Notional: 120000, EventTime: now.Add(-time.Minute)},
		{ID: "e2", Exchange: "okx", Symbol: "ETHUSDT", EventType: "funding", Metadata: map[string]any{"funding_rate": 0.12}, EventTime: now.Add(-2 * time.Minute)},
	}
	aggregator := NewAggregator(fakeAlertLister{items: alerts}, fakeEventLister{items: events}, Config{MaxItems: 1})

	snapshot, err := aggregator.Build(context.Background(), now.Add(-15*time.Minute), now)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if len(snapshot.Alerts) != 1 || len(snapshot.Events) != 1 {
		t.Fatalf("expected bounded snapshot, got %+v", snapshot)
	}
	if snapshot.AlertCount != 2 || snapshot.EventCount != 2 || snapshot.FundingCount != 1 {
		t.Fatalf("unexpected counts: %+v", snapshot)
	}
}
```

- [x] **Step 2: Run failing aggregation test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/summary -run Aggregator -v'
```

Expected: FAIL because `internal/summary` does not exist.

- [x] **Step 3: Add bounded repository filters**

Add `Since time.Time` to `storage.ListFilter`. In `AlertRepo.List`, add `AND created_at >= $N` when `filter.Since` is non-zero. In `MarketEventRepo.List`, add `AND event_time >= $N` when `filter.Since` is non-zero.

- [x] **Step 4: Implement aggregator**

Create:

```go
type Config struct {
	MaxItems int
}

type Snapshot struct {
	WindowFrom   time.Time
	WindowTo     time.Time
	AlertCount   int
	EventCount   int
	FundingCount int
	Alerts       []model.Alert
	Events       []model.MarketEvent
}

func NewAggregator(alerts AlertLister, events EventLister, cfg Config) Aggregator
func (a Aggregator) Build(ctx context.Context, from time.Time, to time.Time) (Snapshot, error)
```

`Build` must request `MaxItems + 1` rows from each repository so it can preserve true bounded sample rows while still reporting whether more data exists through counts derived from fetched rows.

- [x] **Step 5: Run aggregation test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/summary -run Aggregator -v'
```

Expected: PASS.

## Task 4: Summary Generation

**Files:**
- Create: `internal/summary/generator.go`
- Create: `internal/summary/generator_test.go`
- Create: `internal/summary/openai_compatible.go`
- Create: `internal/summary/openai_compatible_test.go`

- [x] **Step 1: Write failing generator tests**

Create tests:

```go
func TestTemplateGeneratorIncludesDisclaimer(t *testing.T)
func TestTemplateGeneratorIncludesFundingContext(t *testing.T)
```

The first test must assert the generated content contains `不构成投资建议`. The second must pass a snapshot with one funding event and assert the content mentions `funding`.

Create OpenAI-compatible generator tests:

```go
func TestOpenAICompatibleGeneratorSendsSnapshotPrompt(t *testing.T)
func TestOpenAICompatibleGeneratorRequiresDisclaimerInResponse(t *testing.T)
```

Use `httptest.NewServer` to assert the request path is `/chat/completions`, the bearer token is present, and the prompt includes alert count, event count, funding count, and `不构成投资建议`.

- [x] **Step 2: Run failing generator tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/summary -run "TemplateGenerator|OpenAICompatible" -v'
```

Expected: FAIL because generator code does not exist.

- [x] **Step 3: Implement generator interface and template generator**

Create:

```go
type Generator interface {
	Generate(context.Context, Snapshot) (string, error)
}

type TemplateGenerator struct {
	Disclaimer string
}

func NewTemplateGenerator(disclaimer string) TemplateGenerator
func (g TemplateGenerator) Generate(ctx context.Context, snapshot Snapshot) (string, error)
```

`Generate` must include window time, alert count, event count, funding count, top sampled alert titles, and the configured disclaimer.

- [x] **Step 4: Implement OpenAI-compatible generator**

Create:

```go
type OpenAICompatibleConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	TimeoutSec int
	Disclaimer string
}

func NewOpenAICompatibleGenerator(cfg OpenAICompatibleConfig, client *http.Client) OpenAICompatibleGenerator
func (g OpenAICompatibleGenerator) Generate(ctx context.Context, snapshot Snapshot) (string, error)
```

`Generate` must POST to `{BaseURL}/chat/completions`, send one system message and one user message, parse the first assistant message, and append the disclaimer if the provider response omits it.

- [x] **Step 5: Run generator tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/summary -run "TemplateGenerator|OpenAICompatible" -v'
```

Expected: PASS.

## Task 5: Summary Service And Scheduler

**Files:**
- Create: `internal/summary/service.go`
- Create: `internal/summary/service_test.go`
- Create: `internal/scheduler/summary_job.go`
- Create: `internal/scheduler/summary_job_test.go`

- [x] **Step 1: Write failing service test**

Create `TestServiceStoresFailedSummaryWhenGenerationFails` proving generator errors produce a `market_summaries` row with `Status: "failed"` and a non-empty `ErrorMessage`.

- [x] **Step 2: Run failing service test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/summary -run Service -v'
```

Expected: FAIL because `Service` does not exist.

- [x] **Step 3: Implement service**

Create:

```go
type SummaryStore interface {
	Insert(context.Context, model.MarketSummary) error
}

type Service struct {
	Aggregator Aggregator
	Generator  Generator
	Store      SummaryStore
	Provider   string
	Window     time.Duration
	Now        func() time.Time
}

func (s Service) RunOnce(ctx context.Context) error
```

`RunOnce` must build the window, generate content, store `generated` on success, store `failed` on generator error, and return only aggregation or storage errors.

- [x] **Step 4: Write scheduler job test**

Create `TestSummaryJobRunOnceDelegatesToRunner` with a fake runner that increments a count and returns nil.

- [x] **Step 5: Implement scheduler job**

Create:

```go
type SummaryRunner interface {
	RunOnce(context.Context) error
}

type SummaryJob struct {
	runner   SummaryRunner
	interval time.Duration
}

func NewSummaryJob(runner SummaryRunner, interval time.Duration) SummaryJob
func (j SummaryJob) RunOnce(ctx context.Context) error
func (j SummaryJob) Start(ctx context.Context)
```

`Start` must ignore `RunOnce` errors so the ticker continues and real-time alert handling is unaffected.

- [x] **Step 6: Run service and scheduler tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/summary ./internal/scheduler -run \"Service|SummaryJob\" -v'
```

Expected: PASS.

## Task 6: Runtime Wiring And Docs

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `cmd/server/main_test.go`
- Modify: `deployments/docker-compose.yml`
- Modify: `README.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`

- [x] **Step 1: Write failing wiring test**

Add `TestBuildSummaryServiceUsesTemplateProvider` and `TestBuildSummaryServiceUsesOpenAICompatibleProvider` to `cmd/server/main_test.go`. The first test should build config with `Summary.Provider = "template"` and assert the returned service is non-nil. The second test should build config with `Summary.Provider = "openai_compatible"` plus base URL, API key, model, and timeout, then assert the returned service is non-nil.

- [x] **Step 2: Implement runtime wiring**

Add a helper in `cmd/server/main.go`:

```go
func buildSummaryService(cfg config.Config, repos *storage.Repositories) summary.Service
```

When `cfg.Summary.Enabled` is true, `main` must start:

```go
summaryJob := scheduler.NewSummaryJob(summaryService, time.Duration(cfg.Summary.IntervalSec)*time.Second)
go summaryJob.Start(ctx)
```

Do not start the job when `cfg.Summary.Enabled` is false.

- [x] **Step 3: Update Docker Compose env**

Add app env defaults:

```yaml
CW_SUMMARY_ENABLED: ${CW_SUMMARY_ENABLED:-false}
CW_SUMMARY_INTERVAL_SEC: ${CW_SUMMARY_INTERVAL_SEC:-900}
CW_SUMMARY_WINDOW_SEC: ${CW_SUMMARY_WINDOW_SEC:-900}
CW_SUMMARY_MAX_ITEMS: ${CW_SUMMARY_MAX_ITEMS:-50}
CW_SUMMARY_PROVIDER: ${CW_SUMMARY_PROVIDER:-template}
CW_SUMMARY_DISCLAIMER: ${CW_SUMMARY_DISCLAIMER:-不构成投资建议}
CW_SUMMARY_API_BASE_URL: ${CW_SUMMARY_API_BASE_URL:-}
CW_SUMMARY_API_KEY: ${CW_SUMMARY_API_KEY:-}
CW_SUMMARY_MODEL: ${CW_SUMMARY_MODEL:-}
CW_SUMMARY_TIMEOUT_SEC: ${CW_SUMMARY_TIMEOUT_SEC:-15}
```

- [x] **Step 4: Update README and master plan**

README must document the ten `summary.*` fields and ten `CW_SUMMARY_*` env vars. It must state that API keys are supplied through env and must not be committed. The master plan should mark AI market summary complete only after the verification gate passes.

- [x] **Step 5: Run runtime tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./cmd/server ./internal/summary ./internal/scheduler -v'
```

Expected: PASS.

## Verification Gate

- [x] `docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'`
- [x] `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh`
- [x] `git diff --check`

## Execution Notes

- 2026-06-29: Plan created after completing `[4]2026-06-29-discord-webhook-notifier.md`; implementation has not started.
- 2026-06-30: Task 2 integration command passed compilation, but the real PostgreSQL/Redis path was skipped because `CW_INTEGRATION_TESTS=1` was not enabled and compose services were not running.
- 2026-06-30: Verification gate passed with full Go tests, Docker Compose smoke on port 18080, and `git diff --check`.
