//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/storage"
)

// TestPostgresRedisRepositoriesExerciseRealDependencies verifies repository and Redis behavior against real containers.
//
// Author: monsterfei
// Date: 2026-06-29
func TestPostgresRedisRepositoriesExerciseRealDependencies(t *testing.T) {
	if os.Getenv("CW_INTEGRATION_TESTS") != "1" {
		t.Skip("set CW_INTEGRATION_TESTS=1 to run real PostgreSQL/Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	postgresDSN := getenvDefault("CW_POSTGRES_DSN", "postgres://postgres:postgres@127.0.0.1:5432/crypto_watchtower?sslmode=disable")
	redisAddr := getenvDefault("CW_REDIS_ADDR", "127.0.0.1:6379")

	postgres, err := storage.NewPostgres(ctx, postgresDSN)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer postgres.Close()

	migrations, err := storage.NewFileMigrationRunner(storage.NewPostgresMigrationDB(postgres), filepath.Join(repoRoot(t), "migrations"))
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if err := migrations.Run(ctx); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	redisClient := storage.NewRedis(redisAddr, os.Getenv("CW_REDIS_PASSWORD"), 0)
	defer func() { _ = redisClient.Close() }()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	repos := storage.NewRepositories(postgres)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	symbol := "INTEG" + suffix
	eventID := "event-" + suffix
	alertID := "alert-" + suffix
	now := time.Now().UTC()

	rule := model.AlertRule{
		Scope:     "system",
		Exchange:  "binance",
		Symbol:    symbol,
		RuleType:  "large_trade",
		Threshold: 123456,
		WindowSec: 60,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repos.AlertRules.UpsertSystemRule(ctx, rule); err != nil {
		t.Fatalf("upsert rule: %v", err)
	}
	rules, err := repos.AlertRules.List(ctx, storage.ListFilter{Symbol: symbol, RuleType: "large_trade", Limit: 1})
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 || rules[0].Symbol != symbol || rules[0].Threshold != rule.Threshold {
		t.Fatalf("unexpected rules: %+v", rules)
	}

	event := model.MarketEvent{
		ID:         eventID,
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     symbol,
		EventType:  "agg_trade",
		Side:       "buy",
		Price:      100,
		Quantity:   2,
		Notional:   200,
		Metadata:   map[string]any{"source": "integration"},
		RawPayload: []byte(`{"source":"integration"}`),
		EventTime:  now,
		CreatedAt:  now,
	}
	if err := repos.MarketEvents.Insert(ctx, event); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	events, err := repos.MarketEvents.List(ctx, storage.ListFilter{Symbol: symbol, EventType: "agg_trade", Limit: 1})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 || events[0].ID != eventID || events[0].Notional != event.Notional {
		t.Fatalf("unexpected events: %+v", events)
	}

	alert := model.Alert{
		ID:          alertID,
		Exchange:    "binance",
		MarketType:  "spot",
		Symbol:      symbol,
		Type:        "large_trade",
		Severity:    "warning",
		Title:       "integration alert",
		Message:     "integration alert message",
		EventID:     eventID,
		TriggerKey:  "binance:" + symbol + ":large_trade",
		TriggerTime: now,
		CreatedAt:   now,
	}
	if err := repos.Alerts.Insert(ctx, alert); err != nil {
		t.Fatalf("insert alert: %v", err)
	}
	alerts, err := repos.Alerts.List(ctx, storage.ListFilter{Symbol: symbol, RuleType: "large_trade", Limit: 1})
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].ID != alertID {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}

	if err := repos.NotificationLogs.Insert(ctx, model.NotificationLog{
		AlertID:   alertID,
		Channel:   "telegram",
		Target:    "integration",
		Status:    "sent",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("insert notification log: %v", err)
	}
	logs, err := repos.NotificationLogs.List(ctx, storage.ListFilter{Status: "sent", Limit: 10})
	if err != nil {
		t.Fatalf("list notification logs: %v", err)
	}
	if !containsNotificationForAlert(logs, alertID) {
		t.Fatalf("expected notification log for %s, got %+v", alertID, logs)
	}

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

	key := "integration:dedupe:" + suffix
	set, err := redisClient.SetNX(ctx, key, "1", time.Minute).Result()
	if err != nil {
		t.Fatalf("set redis key: %v", err)
	}
	if !set {
		t.Fatalf("expected first redis SetNX to set %s", key)
	}
	set, err = redisClient.SetNX(ctx, key, "1", time.Minute).Result()
	if err != nil {
		t.Fatalf("set duplicate redis key: %v", err)
	}
	if set {
		t.Fatalf("expected duplicate redis SetNX to be blocked for %s", key)
	}
	_ = redisClient.Del(ctx, key).Err()
}

// getenvDefault returns an environment value or the provided fallback.
//
// Author: monsterfei
// Date: 2026-06-29
func getenvDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// repoRoot returns the repository root for locating integration fixtures.
//
// Author: monsterfei
// Date: 2026-06-29
func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

// containsNotificationForAlert reports whether logs include the expected alert id.
//
// Author: monsterfei
// Date: 2026-06-29
func containsNotificationForAlert(logs []model.NotificationLog, alertID string) bool {
	for _, item := range logs {
		if item.AlertID == alertID {
			return true
		}
	}
	return false
}
