package rule

import (
	"context"
	"errors"
	"testing"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

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
	if !errors.Is(err, errFakeSender) {
		t.Fatalf("expected sender error, got %v", err)
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
