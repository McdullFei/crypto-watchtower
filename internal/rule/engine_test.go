package rule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

var testEventTime = time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

// TestPipelineLogsEachNotificationChannel verifies each configured sender writes its own log.
//
// Author: monsterfei
// Date: 2026-06-29
func TestPipelineLogsEachNotificationChannel(t *testing.T) {
	repos := &fakePipelineRepositories{}
	alert := model.Alert{
		ID:         "alert-1",
		Exchange:   "binance",
		Symbol:     "BTCUSDT",
		Type:       "large_trade",
		TriggerKey: "binance:BTCUSDT:large_trade",
		EventID:    "event-1",
	}
	pipeline := NewPipeline(
		fakeEvaluator{alerts: []model.Alert{alert}},
		repos,
		nil,
		NewNamedSender("telegram", "default", fakeSender{}),
		NewNamedSender("discord", "https://discord.example/webhook", fakeSender{}),
	)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{ID: "event-1"})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}

	if len(repos.notificationLogs) != 2 {
		t.Fatalf("expected two notification logs, got %#v", repos.notificationLogs)
	}
	if repos.notificationLogs[0].Channel != "telegram" || repos.notificationLogs[0].Target != "default" {
		t.Fatalf("unexpected first log: %#v", repos.notificationLogs[0])
	}
	if repos.notificationLogs[1].Channel != "discord" || repos.notificationLogs[1].Target != "https://discord.example/webhook" {
		t.Fatalf("unexpected second log: %#v", repos.notificationLogs[1])
	}
}

// TestPipelineContinuesLoggingAfterSenderFailure verifies later channels are logged after an earlier send fails.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-07-03
func TestPipelineContinuesLoggingAfterSenderFailure(t *testing.T) {
	repos := &fakePipelineRepositories{}
	alert := model.Alert{
		ID:         "alert-1",
		Exchange:   "binance",
		Symbol:     "BTCUSDT",
		Type:       "large_trade",
		TriggerKey: "binance:BTCUSDT:large_trade",
		EventID:    "event-1",
	}
	pipeline := NewPipeline(
		fakeEvaluator{alerts: []model.Alert{alert}},
		repos,
		nil,
		NewNamedSender("telegram", "default", fakeSender{err: errFakeSender}),
		NewNamedSender("discord", "https://discord.example/webhook", fakeSender{}),
	)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{ID: "event-1"})
	if err != nil {
		t.Fatalf("expected sender error to be logged without failing event handling, got %v", err)
	}

	if len(repos.notificationLogs) != 2 {
		t.Fatalf("expected two notification logs, got %#v", repos.notificationLogs)
	}
	if repos.notificationLogs[0].Channel != "telegram" || repos.notificationLogs[0].Status != "failed" || repos.notificationLogs[0].ErrorMessage == "" {
		t.Fatalf("unexpected failed log: %#v", repos.notificationLogs[0])
	}
	if repos.notificationLogs[1].Channel != "discord" || repos.notificationLogs[1].Status != "sent" || repos.notificationLogs[1].ErrorMessage != "" {
		t.Fatalf("unexpected successful log: %#v", repos.notificationLogs[1])
	}
}

// TestPipelineDoesNotFailEventWhenSenderFails verifies notification delivery errors stay isolated from event handling.
//
// Author: monsterfei
// Date: 2026-07-03
func TestPipelineDoesNotFailEventWhenSenderFails(t *testing.T) {
	repos := &fakePipelineRepositories{}
	alert := model.Alert{
		ID:         "alert-1",
		Exchange:   "binance",
		Symbol:     "BTCUSDT",
		Type:       "large_trade",
		TriggerKey: "binance:BTCUSDT:large_trade",
		EventID:    "event-1",
	}
	pipeline := NewPipeline(
		fakeEvaluator{alerts: []model.Alert{alert}},
		repos,
		nil,
		NewNamedSender("discord", "https://discord.example/webhook", fakeSender{err: errFakeSender}),
	)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{ID: "event-1"})
	if err != nil {
		t.Fatalf("expected sender error to be logged without failing event handling, got %v", err)
	}
	if len(repos.alerts) != 1 || len(repos.notificationLogs) != 1 {
		t.Fatalf("expected alert and notification log to be stored, got alerts=%+v logs=%+v", repos.alerts, repos.notificationLogs)
	}
	if repos.notificationLogs[0].Status != "failed" || repos.notificationLogs[0].ErrorMessage == "" {
		t.Fatalf("expected failed notification log, got %+v", repos.notificationLogs[0])
	}
}

// TestPipelineDeliversMatchedUserRulesToBoundUsers verifies user rule fanout writes user notification logs.
//
// Author: monsterfei
// Date: 2026-07-01
func TestPipelineDeliversMatchedUserRulesToBoundUsers(t *testing.T) {
	userID := int64(42)
	ruleID := int64(7)
	repos := &fakePipelineRepositories{}
	userRules := fakeUserRuleRepository{
		targets: []UserRuleTarget{{
			User: model.User{
				ID:                      userID,
				Status:                  model.UserStatusActive,
				TelegramChatID:          "12345",
				TelegramDeliveryEnabled: true,
			},
			Rule: model.AlertRule{
				ID:        ruleID,
				UserID:    &userID,
				Scope:     "user",
				Exchange:  "binance",
				Symbol:    "BTCUSDT",
				RuleType:  "large_trade",
				Threshold: 100000,
				Enabled:   true,
			},
		}},
	}
	userSender := &fakeUserAlertSender{}
	pipeline := NewPipeline(fakeEvaluator{}, repos, nil).
		WithUserFanout(userRules, "telegram", userSender)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{
		ID:         "event-user-1",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Side:       "buy",
		Price:      100000,
		Quantity:   2,
		Notional:   200000,
	})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(userSender.targets) != 1 || userSender.targets[0] != "12345" {
		t.Fatalf("expected user telegram send, got %+v", userSender.targets)
	}
	if len(repos.notificationLogs) != 1 || repos.notificationLogs[0].UserID == nil || *repos.notificationLogs[0].UserID != userID {
		t.Fatalf("expected user notification log, got %+v", repos.notificationLogs)
	}
	if repos.notificationLogs[0].Channel != "telegram" || repos.notificationLogs[0].Target != "12345" {
		t.Fatalf("unexpected user notification log: %+v", repos.notificationLogs[0])
	}
	if len(repos.alerts) != 1 || repos.alerts[0].RuleID != "user:7" {
		t.Fatalf("expected user alert with rule id, got %+v", repos.alerts)
	}
}

// TestPipelineSkipsUnboundOrDisabledUserRuleTargets verifies invalid user targets do not send notifications.
//
// Author: monsterfei
// Date: 2026-07-01
func TestPipelineSkipsUnboundOrDisabledUserRuleTargets(t *testing.T) {
	userID := int64(42)
	repos := &fakePipelineRepositories{}
	userRules := fakeUserRuleRepository{
		targets: []UserRuleTarget{
			{
				User: model.User{ID: userID, Status: model.UserStatusActive},
				Rule: model.AlertRule{ID: 1, UserID: &userID, Scope: "user", Exchange: "binance", Symbol: "BTCUSDT", RuleType: "large_trade", Threshold: 100000, Enabled: true},
			},
			{
				User: model.User{ID: 43, Status: model.UserStatusDisabled, TelegramChatID: "54321"},
				Rule: model.AlertRule{ID: 2, UserID: &userID, Scope: "user", Exchange: "binance", Symbol: "BTCUSDT", RuleType: "large_trade", Threshold: 100000, Enabled: true},
			},
		},
	}
	userSender := &fakeUserAlertSender{}
	pipeline := NewPipeline(fakeEvaluator{}, repos, nil).
		WithUserFanout(userRules, "telegram", userSender)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{
		ID:        "event-user-2",
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		EventType: "agg_trade",
		Notional:  200000,
	})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(userSender.targets) != 0 || len(repos.notificationLogs) != 0 {
		t.Fatalf("expected no invalid user sends, targets=%+v logs=%+v", userSender.targets, repos.notificationLogs)
	}
}

// TestPipelineDeliversUserLargeTradeWindowRules verifies user window rules use isolated fanout evaluation.
//
// Author: monsterfei
// Date: 2026-07-01
func TestPipelineDeliversUserLargeTradeWindowRules(t *testing.T) {
	userID := int64(42)
	ruleID := int64(9)
	repos := &fakePipelineRepositories{}
	userRules := fakeUserRuleRepository{
		targets: []UserRuleTarget{{
			User: model.User{
				ID:                      userID,
				Status:                  model.UserStatusActive,
				TelegramChatID:          "12345",
				TelegramDeliveryEnabled: true,
			},
			Rule: model.AlertRule{
				ID:        ruleID,
				UserID:    &userID,
				Scope:     "user",
				Exchange:  "binance",
				Symbol:    "BTCUSDT",
				RuleType:  "large_trade_window",
				Threshold: 300000,
				WindowSec: 60,
				Enabled:   true,
			},
		}},
	}
	userSender := &fakeUserAlertSender{}
	pipeline := NewPipeline(fakeEvaluator{}, repos, nil).
		WithUserFanout(userRules, "telegram", userSender)

	firstEvent := model.MarketEvent{
		ID:         "event-window-1",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Notional:   150000,
		EventTime:  testEventTime,
	}
	if err := pipeline.HandleEvent(context.Background(), firstEvent); err != nil {
		t.Fatalf("handle first event: %v", err)
	}
	if len(userSender.targets) != 0 {
		t.Fatalf("did not expect delivery before threshold, got %+v", userSender.targets)
	}

	secondEvent := firstEvent
	secondEvent.ID = "event-window-2"
	secondEvent.Notional = 200000
	secondEvent.EventTime = testEventTime.Add(30 * time.Second)
	if err := pipeline.HandleEvent(context.Background(), secondEvent); err != nil {
		t.Fatalf("handle second event: %v", err)
	}

	if len(userSender.targets) != 1 || userSender.targets[0] != "12345" {
		t.Fatalf("expected one user window delivery, got %+v", userSender.targets)
	}
	if len(repos.alerts) != 1 {
		t.Fatalf("expected one user window alert, got %+v", repos.alerts)
	}
	alert := repos.alerts[0]
	if alert.Type != "large_trade_window" || alert.RuleID != "user:9" {
		t.Fatalf("unexpected user window alert: %+v", alert)
	}
	if alert.TriggerKey == secondEvent.TriggerBucket("large_trade_window") {
		t.Fatalf("expected user-isolated trigger key, got %q", alert.TriggerKey)
	}
}

// TestPipelineRecordsDisabledTelegramDelivery verifies disabled user delivery skips sends but logs intent.
//
// Author: monsterfei
// Date: 2026-07-01
func TestPipelineRecordsDisabledTelegramDelivery(t *testing.T) {
	userID := int64(42)
	repos := &fakePipelineRepositories{}
	userRules := fakeUserRuleRepository{
		targets: []UserRuleTarget{{
			User: model.User{
				ID:                      userID,
				Status:                  model.UserStatusActive,
				TelegramChatID:          "12345",
				TelegramDeliveryEnabled: false,
			},
			Rule: model.AlertRule{
				ID:        11,
				UserID:    &userID,
				Scope:     "user",
				Exchange:  "binance",
				Symbol:    "BTCUSDT",
				RuleType:  "large_trade",
				Threshold: 100000,
				Enabled:   true,
			},
		}},
	}
	userSender := &fakeUserAlertSender{}
	pipeline := NewPipeline(fakeEvaluator{}, repos, nil).
		WithUserFanout(userRules, "telegram", userSender)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{
		ID:        "event-disabled-delivery",
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		EventType: "agg_trade",
		Notional:  200000,
		EventTime: testEventTime,
	})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(userSender.targets) != 0 {
		t.Fatalf("expected no Telegram send when delivery disabled, got %+v", userSender.targets)
	}
	if len(repos.notificationLogs) != 1 {
		t.Fatalf("expected one disabled notification log, got %+v", repos.notificationLogs)
	}
	log := repos.notificationLogs[0]
	if log.Status != "disabled" || log.Target != "12345" || log.UserID == nil || *log.UserID != userID {
		t.Fatalf("unexpected disabled delivery log: %+v", log)
	}
}

// TestPipelineRecordsQuietHoursWithoutSending verifies user quiet hours suppress Telegram delivery.
//
// Author: monsterfei
// Date: 2026-07-01
func TestPipelineRecordsQuietHoursWithoutSending(t *testing.T) {
	userID := int64(42)
	repos := &fakePipelineRepositories{}
	userRules := fakeUserRuleRepository{
		targets: []UserRuleTarget{{
			User: model.User{
				ID:                         userID,
				Status:                     model.UserStatusActive,
				TelegramChatID:             "12345",
				TelegramDeliveryEnabled:    true,
				TelegramQuietHoursEnabled:  true,
				TelegramQuietHoursStart:    "23:00",
				TelegramQuietHoursEnd:      "07:00",
				TelegramQuietHoursTimezone: "Asia/Shanghai",
			},
			Rule: model.AlertRule{
				ID:        12,
				UserID:    &userID,
				Scope:     "user",
				Exchange:  "binance",
				Symbol:    "BTCUSDT",
				RuleType:  "large_trade",
				Threshold: 100000,
				Enabled:   true,
			},
		}},
	}
	userSender := &fakeUserAlertSender{}
	pipeline := NewPipeline(fakeEvaluator{}, repos, nil).
		WithUserFanout(userRules, "telegram", userSender)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{
		ID:        "event-quiet-hours",
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		EventType: "agg_trade",
		Notional:  200000,
		EventTime: time.Date(2026, 7, 1, 16, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(userSender.targets) != 0 {
		t.Fatalf("expected quiet hours to suppress Telegram sends, got %+v", userSender.targets)
	}
	if len(repos.notificationLogs) != 1 {
		t.Fatalf("expected one quiet-hours notification log, got %+v", repos.notificationLogs)
	}
	log := repos.notificationLogs[0]
	if log.Status != "quiet_hours" || log.Target != "12345" || log.UserID == nil || *log.UserID != userID {
		t.Fatalf("unexpected quiet-hours log: %+v", log)
	}
}

// TestPipelineQueuesAndFlushesDigest verifies digest mode queues bounded alerts and later sends a summary.
//
// Author: monsterfei
// Date: 2026-07-01
func TestPipelineQueuesAndFlushesDigest(t *testing.T) {
	userID := int64(42)
	repos := &fakePipelineRepositories{}
	userRules := fakeUserRuleRepository{
		targets: []UserRuleTarget{{
			User: model.User{
				ID:                        userID,
				Status:                    model.UserStatusActive,
				TelegramChatID:            "12345",
				TelegramDeliveryEnabled:   true,
				TelegramDigestEnabled:     true,
				TelegramDigestIntervalMin: 30,
			},
			Rule: model.AlertRule{
				ID:        13,
				UserID:    &userID,
				Scope:     "user",
				Exchange:  "binance",
				Symbol:    "BTCUSDT",
				RuleType:  "large_trade",
				Threshold: 100000,
				Enabled:   true,
			},
		}},
	}
	userSender := &fakeUserAlertSender{}
	pipeline := NewPipeline(fakeEvaluator{}, repos, nil).
		WithUserFanout(userRules, "telegram", userSender)
	eventTime := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	err := pipeline.HandleEvent(context.Background(), model.MarketEvent{
		ID:        "event-digest",
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		EventType: "agg_trade",
		Notional:  200000,
		EventTime: eventTime,
	})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(userSender.targets) != 0 {
		t.Fatalf("expected digest mode to queue without immediate sends, got %+v", userSender.targets)
	}
	if len(repos.notificationLogs) != 1 || repos.notificationLogs[0].Status != "digested" {
		t.Fatalf("expected digested notification log, got %+v", repos.notificationLogs)
	}

	if err := pipeline.FlushUserDigests(context.Background(), eventTime.Add(31*time.Minute)); err != nil {
		t.Fatalf("flush digest: %v", err)
	}
	if len(userSender.targets) != 1 || userSender.targets[0] != "12345" {
		t.Fatalf("expected one digest Telegram send, got %+v", userSender.targets)
	}
	if len(userSender.alerts) != 1 || userSender.alerts[0].Type != "user_digest" {
		t.Fatalf("expected digest summary alert, got %+v", userSender.alerts)
	}
}

type fakeEvaluator struct {
	alerts []model.Alert
}

func (e fakeEvaluator) Evaluate(model.MarketEvent) []model.Alert {
	return e.alerts
}

type fakeSender struct {
	err error
}

func (s fakeSender) Send(context.Context, model.Alert) error {
	return s.err
}

type fakePipelineRepositories struct {
	marketEvents     []model.MarketEvent
	alerts           []model.Alert
	notificationLogs []model.NotificationLog
	err              error
}

type fakeUserRuleRepository struct {
	targets []UserRuleTarget
}

func (r fakeUserRuleRepository) ListActiveUserRulesForEvent(context.Context, model.MarketEvent, int) ([]UserRuleTarget, error) {
	return r.targets, nil
}

// fakeUserAlertSender records user-targeted sends for rule pipeline tests.
//
// Author: monsterfei
// Date: 2026-07-01
type fakeUserAlertSender struct {
	targets []string
	alerts  []model.Alert
	err     error
}

// SendTo records one targeted alert send for rule pipeline tests.
//
// Author: monsterfei
// Date: 2026-07-01
// @param target User notification target.
// @param alert Alert sent to the target.
// @returns Configured fake sender error.
func (s *fakeUserAlertSender) SendTo(ctx context.Context, target string, alert model.Alert) error {
	s.targets = append(s.targets, target)
	s.alerts = append(s.alerts, alert)
	return s.err
}

func (r *fakePipelineRepositories) InsertMarketEvent(_ context.Context, event model.MarketEvent) error {
	if r.err != nil {
		return r.err
	}
	r.marketEvents = append(r.marketEvents, event)
	return nil
}

func (r *fakePipelineRepositories) InsertAlert(_ context.Context, alert model.Alert) error {
	if r.err != nil {
		return r.err
	}
	r.alerts = append(r.alerts, alert)
	return nil
}

func (r *fakePipelineRepositories) InsertNotificationLog(_ context.Context, log model.NotificationLog) error {
	if r.err != nil {
		return r.err
	}
	r.notificationLogs = append(r.notificationLogs, log)
	return nil
}

var errFakePipelineRepositories = errors.New("fake repository error")
var errFakeSender = errors.New("fake sender error")
