package summary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// TestServiceStoresFailedSummaryWhenGenerationFails verifies generator errors are persisted but isolated.
//
// Author: monsterfei
// Date: 2026-06-30
func TestServiceStoresFailedSummaryWhenGenerationFails(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 15, 0, 0, time.UTC)
	store := &fakeSummaryStore{}
	service := Service{
		Aggregator: NewAggregator(fakeAlertLister{}, fakeEventLister{}, Config{MaxItems: 1}),
		Generator:  failingGenerator{},
		Store:      store,
		Provider:   "template",
		Window:     15 * time.Minute,
		Now:        func() time.Time { return now },
	}

	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatalf("expected generator error to be isolated, got %v", err)
	}
	if len(store.items) != 1 {
		t.Fatalf("expected one stored summary, got %+v", store.items)
	}
	if store.items[0].Status != "failed" || store.items[0].ErrorMessage == "" {
		t.Fatalf("expected failed summary with error message, got %+v", store.items[0])
	}
}

// TestServiceSendsGeneratedSummaryToNotificationSenders verifies generated summaries are observable through notification logs.
//
// Author: __AUTHOR__
// Date: 2026-07-03
func TestServiceSendsGeneratedSummaryToNotificationSenders(t *testing.T) {
	now := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	store := &fakeSummaryStore{}
	notifications := &fakeSummaryNotificationStore{}
	sender := &fakeSummarySender{name: "telegram", target: "default"}
	service := Service{
		Aggregator:    NewAggregator(fakeAlertLister{}, fakeEventLister{}, Config{MaxItems: 1}),
		Generator:     staticGenerator{content: "BTCUSDT 波动放大\n不构成投资建议"},
		Store:         store,
		Notifications: notifications,
		Senders:       []NamedSender{sender},
		Provider:      "template",
		Window:        15 * time.Minute,
		Now:           func() time.Time { return now },
	}

	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatalf("run summary service: %v", err)
	}
	if len(sender.alerts) != 1 {
		t.Fatalf("expected one summary notification send, got %+v", sender.alerts)
	}
	if sender.alerts[0].Type != "market_summary" || sender.alerts[0].Message != "BTCUSDT 波动放大\n不构成投资建议" {
		t.Fatalf("unexpected summary alert: %+v", sender.alerts[0])
	}
	if len(notifications.logs) != 1 {
		t.Fatalf("expected one notification log, got %+v", notifications.logs)
	}
	if notifications.logs[0].AlertID != store.items[0].ID || notifications.logs[0].Channel != "telegram" || notifications.logs[0].Status != "sent" {
		t.Fatalf("unexpected notification log: %+v", notifications.logs[0])
	}
}

// staticGenerator returns a fixed successful summary for service tests.
//
// Author: __AUTHOR__
// Date: 2026-07-03
type staticGenerator struct {
	content string
}

// Generate returns configured summary content for service tests.
//
// Author: __AUTHOR__
// Date: 2026-07-03
// @param ctx Request context.
// @param snapshot Bounded market snapshot.
// @returns Static summary content.
func (g staticGenerator) Generate(context.Context, Snapshot) (string, error) {
	return g.content, nil
}

type failingGenerator struct{}

// Generate returns a deterministic failure for service tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (failingGenerator) Generate(context.Context, Snapshot) (string, error) {
	return "", errors.New("provider failed")
}

type fakeSummaryStore struct {
	items []model.MarketSummary
}

// Insert records summaries in memory for service tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (f *fakeSummaryStore) Insert(_ context.Context, summary model.MarketSummary) error {
	f.items = append(f.items, summary)
	return nil
}

// fakeSummaryNotificationStore records summary notification logs in memory.
//
// Author: __AUTHOR__
// Date: 2026-07-03
type fakeSummaryNotificationStore struct {
	logs []model.NotificationLog
}

// InsertNotificationLog records summary notification logs in memory for service tests.
//
// Author: __AUTHOR__
// Date: 2026-07-03
// @param ctx Request context.
// @param log Notification delivery log.
// @returns Error when recording fails.
func (f *fakeSummaryNotificationStore) InsertNotificationLog(_ context.Context, log model.NotificationLog) error {
	f.logs = append(f.logs, log)
	return nil
}

// fakeSummarySender records summary alerts sent by the service.
//
// Author: __AUTHOR__
// Date: 2026-07-03
type fakeSummarySender struct {
	name   string
	target string
	alerts []model.Alert
}

// Name returns the fake sender channel name.
//
// Author: __AUTHOR__
// Date: 2026-07-03
// @returns Channel name.
func (f fakeSummarySender) Name() string {
	return f.name
}

// Target returns the fake sender target.
//
// Author: __AUTHOR__
// Date: 2026-07-03
// @returns Target identifier.
func (f fakeSummarySender) Target() string {
	return f.target
}

// Send records the alert sent by the summary service.
//
// Author: __AUTHOR__
// Date: 2026-07-03
// @param ctx Request context.
// @param alert Summary alert to send.
// @returns Error when sending fails.
func (f *fakeSummarySender) Send(_ context.Context, alert model.Alert) error {
	f.alerts = append(f.alerts, alert)
	return nil
}
