//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

	var userID int64
	if err := postgres.QueryRow(ctx, `
		INSERT INTO users (email, created_at, updated_at)
		VALUES ($1, $2, $3)
		RETURNING id
	`, "integration-"+suffix+"@example.test", now, now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userRule := model.AlertRule{
		UserID:    &userID,
		Scope:     "user",
		Exchange:  "binance",
		Symbol:    symbol,
		RuleType:  "large_trade",
		Threshold: 234567,
		WindowSec: 60,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repos.AlertRules.UpsertUserRule(ctx, userRule); err != nil {
		t.Fatalf("upsert user rule: %v", err)
	}
	userRules, err := repos.AlertRules.List(ctx, storage.ListFilter{Scope: "user", UserID: &userID, Limit: 1})
	if err != nil {
		t.Fatalf("list user rules: %v", err)
	}
	if len(userRules) != 1 || userRules[0].UserID == nil || *userRules[0].UserID != userID || userRules[0].Threshold != userRule.Threshold {
		t.Fatalf("unexpected user rules: %+v", userRules)
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

// TestAuthRepositoriesPersistSessionsAndResetTokens verifies auth persistence against PostgreSQL.
//
// Author: monsterfei
// Date: 2026-06-30
func TestAuthRepositoriesPersistSessionsAndResetTokens(t *testing.T) {
	ctx := context.Background()
	repos := setupIntegrationRepositories(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	now := time.Now().UTC()

	user, err := repos.Users.CreateWithPassword(ctx, model.User{
		Email:        "auth-user-" + suffix + "@example.com",
		PasswordHash: "bcrypt-hash",
		Plan:         model.UserPlanFree,
		Status:       model.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	found, ok, err := repos.Users.FindByEmail(ctx, strings.ToUpper(user.Email))
	if err != nil || !ok || found.ID != user.ID || found.PasswordHash != user.PasswordHash {
		t.Fatalf("find user by email: user=%+v ok=%v err=%v", found, ok, err)
	}
	expiresAt := now.Add(time.Hour)
	sessionHash := strings.Repeat("a", 63) + suffix[len(suffix)-1:]
	if err := repos.Sessions.Create(ctx, model.UserSession{
		UserID:    user.ID,
		TokenHash: sessionHash,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	session, ok, err := repos.Sessions.FindActiveByHash(ctx, sessionHash, time.Now().UTC())
	if err != nil || !ok || session.UserID != user.ID {
		t.Fatalf("find active session: session=%+v ok=%v err=%v", session, ok, err)
	}
	if err := repos.Sessions.RevokeByHash(ctx, sessionHash, time.Now().UTC()); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, ok, err := repos.Sessions.FindActiveByHash(ctx, sessionHash, time.Now().UTC()); err != nil || ok {
		t.Fatalf("expected revoked session to be inactive, ok=%v err=%v", ok, err)
	}

	resetHash := strings.Repeat("b", 63) + suffix[len(suffix)-1:]
	if err := repos.PasswordResetTokens.Create(ctx, model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: resetHash,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create reset token: %v", err)
	}
	resetToken, ok, err := repos.PasswordResetTokens.FindActiveByHash(ctx, resetHash, time.Now().UTC())
	if err != nil || !ok || resetToken.UserID != user.ID {
		t.Fatalf("find active reset token: token=%+v ok=%v err=%v", resetToken, ok, err)
	}
	if err := repos.PasswordResetTokens.MarkUsed(ctx, resetHash, time.Now().UTC()); err != nil {
		t.Fatalf("mark reset token used: %v", err)
	}
	if _, ok, err := repos.PasswordResetTokens.FindActiveByHash(ctx, resetHash, time.Now().UTC()); err != nil || ok {
		t.Fatalf("expected used reset token to be inactive, ok=%v err=%v", ok, err)
	}
}

// TestTelegramBindingRepositoriesPersistAndConsumeTokens verifies binding tokens and account chat ids persist.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramBindingRepositoriesPersistAndConsumeTokens(t *testing.T) {
	ctx := context.Background()
	repos := setupIntegrationRepositories(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	now := time.Now().UTC()

	user, err := repos.Users.CreateWithPassword(ctx, model.User{
		Email:        "telegram-binding-" + suffix + "@example.com",
		PasswordHash: "bcrypt-hash",
		Plan:         model.UserPlanFree,
		Status:       model.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenHash := strings.Repeat("c", 63) + suffix[len(suffix)-1:]
	if err := repos.TelegramBindingTokens.Create(ctx, model.TelegramBindingToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create binding token: %v", err)
	}
	token, ok, err := repos.TelegramBindingTokens.FindActiveByHash(ctx, tokenHash, now)
	if err != nil || !ok || token.UserID != user.ID {
		t.Fatalf("find binding token: token=%+v ok=%v err=%v", token, ok, err)
	}
	chatID := "12345" + suffix
	if err := repos.Users.BindTelegramChat(ctx, token.UserID, chatID); err != nil {
		t.Fatalf("bind telegram chat: %v", err)
	}
	if err := repos.TelegramBindingTokens.MarkUsed(ctx, tokenHash, now); err != nil {
		t.Fatalf("mark binding token used: %v", err)
	}
	if _, ok, err := repos.TelegramBindingTokens.FindActiveByHash(ctx, tokenHash, now); err != nil || ok {
		t.Fatalf("expected used binding token inactive, ok=%v err=%v", ok, err)
	}
	found, ok, err := repos.Users.FindByID(ctx, user.ID)
	if err != nil || !ok || found.TelegramChatID != chatID {
		t.Fatalf("find bound user: user=%+v ok=%v err=%v", found, ok, err)
	}
}

// TestUserDeliveryPreferenceRepositoryPersistsTelegramSwitch verifies Telegram delivery preference persistence.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserDeliveryPreferenceRepositoryPersistsTelegramSwitch(t *testing.T) {
	ctx := context.Background()
	repos := setupIntegrationRepositories(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	now := time.Now().UTC()

	user, err := repos.Users.CreateWithPassword(ctx, model.User{
		Email:        "delivery-pref-" + suffix + "@example.com",
		PasswordHash: "bcrypt-hash",
		Plan:         model.UserPlanFree,
		Status:       model.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if !user.TelegramDeliveryEnabled {
		t.Fatalf("expected telegram delivery enabled by default, got %+v", user)
	}

	if err := repos.Users.UpdateTelegramDeliveryEnabled(ctx, user.ID, false); err != nil {
		t.Fatalf("disable telegram delivery: %v", err)
	}
	found, ok, err := repos.Users.FindByID(ctx, user.ID)
	if err != nil || !ok {
		t.Fatalf("find user: user=%+v ok=%v err=%v", found, ok, err)
	}
	if found.TelegramDeliveryEnabled {
		t.Fatalf("expected telegram delivery disabled, got %+v", found)
	}

	alertID := "delivery-pref-alert-" + suffix
	if err := repos.Alerts.Insert(ctx, model.Alert{
		ID:          alertID,
		Exchange:    "binance",
		MarketType:  "spot",
		Symbol:      "PREF" + suffix,
		Type:        "large_trade",
		Severity:    "warning",
		Title:       "delivery preference alert",
		Message:     "delivery preference alert",
		EventID:     "delivery-pref-event-" + suffix,
		TriggerKey:  "binance:PREF" + suffix + ":large_trade",
		TriggerTime: now,
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("insert alert: %v", err)
	}
	if err := repos.NotificationLogs.Insert(ctx, model.NotificationLog{
		UserID:    &user.ID,
		AlertID:   alertID,
		Channel:   "telegram",
		Target:    "12345",
		Status:    "disabled",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("insert notification log: %v", err)
	}
	logs, err := repos.NotificationLogs.LatestForUser(ctx, user.ID, 1)
	if err != nil {
		t.Fatalf("latest notification logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Status != "disabled" || logs[0].UserID == nil || *logs[0].UserID != user.ID {
		t.Fatalf("unexpected latest notification logs: %+v", logs)
	}
}

// TestUserNotificationPreferencesRepositoryPersistsQuietHoursAndDigest verifies quiet-hours and digest preferences persist.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserNotificationPreferencesRepositoryPersistsQuietHoursAndDigest(t *testing.T) {
	ctx := context.Background()
	repos := setupIntegrationRepositories(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	now := time.Now().UTC()

	user, err := repos.Users.CreateWithPassword(ctx, model.User{
		Email:        "notification-pref-" + suffix + "@example.com",
		PasswordHash: "bcrypt-hash",
		Plan:         model.UserPlanFree,
		Status:       model.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.TelegramQuietHoursEnabled || user.TelegramDigestEnabled {
		t.Fatalf("expected quiet hours and digest disabled by default, got %+v", user)
	}

	preferences := model.UserNotificationPreferences{
		TelegramQuietHoursEnabled:  true,
		TelegramQuietHoursStart:    "23:30",
		TelegramQuietHoursEnd:      "07:15",
		TelegramQuietHoursTimezone: "Asia/Shanghai",
		TelegramDigestEnabled:      true,
		TelegramDigestIntervalMin:  45,
	}
	if err := repos.Users.UpdateTelegramNotificationPreferences(ctx, user.ID, preferences); err != nil {
		t.Fatalf("update notification preferences: %v", err)
	}
	found, ok, err := repos.Users.FindByID(ctx, user.ID)
	if err != nil || !ok {
		t.Fatalf("find user: user=%+v ok=%v err=%v", found, ok, err)
	}
	if !found.TelegramQuietHoursEnabled || found.TelegramQuietHoursStart != "23:30" || found.TelegramQuietHoursEnd != "07:15" || found.TelegramQuietHoursTimezone != "Asia/Shanghai" {
		t.Fatalf("unexpected quiet-hours preferences: %+v", found)
	}
	if !found.TelegramDigestEnabled || found.TelegramDigestIntervalMin != 45 {
		t.Fatalf("unexpected digest preferences: %+v", found)
	}
}

// TestUserTelegramUnbindClearsChatAndPreservesDeliveryPreference verifies unbind persistence.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserTelegramUnbindClearsChatAndPreservesDeliveryPreference(t *testing.T) {
	ctx := context.Background()
	repos := setupIntegrationRepositories(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	now := time.Now().UTC()

	user, err := repos.Users.CreateWithPassword(ctx, model.User{
		Email:          "unbind-" + suffix + "@example.com",
		PasswordHash:   "bcrypt-hash",
		TelegramChatID: "unbind-chat-" + suffix,
		Plan:           model.UserPlanFree,
		Status:         model.UserStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := repos.Users.UpdateTelegramDeliveryEnabled(ctx, user.ID, false); err != nil {
		t.Fatalf("disable telegram delivery: %v", err)
	}
	if err := repos.Users.UnbindTelegramChat(ctx, user.ID); err != nil {
		t.Fatalf("unbind telegram chat: %v", err)
	}
	found, ok, err := repos.Users.FindByID(ctx, user.ID)
	if err != nil || !ok {
		t.Fatalf("find user: user=%+v ok=%v err=%v", found, ok, err)
	}
	if found.TelegramChatID != "" {
		t.Fatalf("expected telegram chat cleared, got %+v", found)
	}
	if found.TelegramDeliveryEnabled {
		t.Fatalf("expected telegram delivery preference preserved as disabled, got %+v", found)
	}
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

// setupIntegrationRepositories prepares migrated PostgreSQL repositories for integration tests.
//
// Author: monsterfei
// Date: 2026-06-30
func setupIntegrationRepositories(t *testing.T) *storage.Repositories {
	t.Helper()
	if os.Getenv("CW_INTEGRATION_TESTS") != "1" {
		t.Skip("set CW_INTEGRATION_TESTS=1 to run real PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	postgresDSN := getenvDefault("CW_POSTGRES_DSN", "postgres://postgres:postgres@127.0.0.1:5432/crypto_watchtower?sslmode=disable")
	postgres, err := storage.NewPostgres(ctx, postgresDSN)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(postgres.Close)

	migrations, err := storage.NewFileMigrationRunner(storage.NewPostgresMigrationDB(postgres), filepath.Join(repoRoot(t), "migrations"))
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if err := migrations.Run(ctx); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return storage.NewRepositories(postgres)
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
