package summary

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

const (
	summaryStatusGenerated = "generated"
	summaryStatusFailed    = "failed"
)

// SummaryStore persists generated market summary records.
//
// Author: monsterfei
// Date: 2026-06-30
type SummaryStore interface {
	Insert(context.Context, model.MarketSummary) error
}

// NotificationLogStore persists market summary notification delivery records.
//
// Author: monsterfei
// Date: 2026-07-03
type NotificationLogStore interface {
	InsertNotificationLog(context.Context, model.NotificationLog) error
}

// NamedSender sends generated summaries and exposes notification log metadata.
//
// Author: monsterfei
// Date: 2026-07-03
type NamedSender interface {
	Name() string
	Target() string
	Send(context.Context, model.Alert) error
}

// Service orchestrates one bounded summary generation run.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-07-03
type Service struct {
	Aggregator    Aggregator
	Generator     Generator
	Store         SummaryStore
	Notifications NotificationLogStore
	Senders       []NamedSender
	Provider      string
	Window        time.Duration
	Now           func() time.Time
}

// RunOnce builds, generates, and stores one market summary.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-07-03
// @param ctx Request context.
// @returns Error for aggregation or storage failures.
func (s Service) RunOnce(ctx context.Context) error {
	now := s.now()
	window := s.Window
	if window <= 0 {
		window = 15 * time.Minute
	}
	from := now.Add(-window)

	snapshot, err := s.Aggregator.Build(ctx, from, now)
	if err != nil {
		return err
	}
	content, genErr := s.Generator.Generate(ctx, snapshot)

	summary := model.MarketSummary{
		ID:         fmt.Sprintf("summary-%d", now.UnixNano()),
		WindowFrom: from,
		WindowTo:   now,
		Provider:   s.Provider,
		Status:     summaryStatusGenerated,
		Content:    content,
		CreatedAt:  now,
	}
	if genErr != nil {
		slog.Error("generate market summary", "err", genErr)
		summary.Status = summaryStatusFailed
		summary.Content = ""
		summary.ErrorMessage = genErr.Error()
	}
	if err := s.Store.Insert(ctx, summary); err != nil {
		return err
	}
	if summary.Status == summaryStatusGenerated {
		if err := s.sendSummary(ctx, summary); err != nil {
			return err
		}
	}
	return nil
}

// sendSummary delivers one generated summary through optional notification senders.
//
// Author: monsterfei
// Date: 2026-07-03
// @param ctx Request context.
// @param summary Persisted summary to deliver.
// @returns Error when notification log persistence fails.
func (s Service) sendSummary(ctx context.Context, summary model.MarketSummary) error {
	if s.Notifications == nil || len(s.Senders) == 0 || summary.Content == "" {
		return nil
	}
	alert := summaryAlert(summary)
	for _, sender := range s.Senders {
		sendErr := sender.Send(ctx, alert)
		status := "sent"
		message := ""
		if sendErr != nil {
			status = "failed"
			message = sendErr.Error()
		}
		if err := s.Notifications.InsertNotificationLog(ctx, model.NotificationLog{
			AlertID:      summary.ID,
			Channel:      sender.Name(),
			Target:       sender.Target(),
			Status:       status,
			ErrorMessage: message,
			CreatedAt:    summary.CreatedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

// summaryAlert adapts generated summary content to the existing notifier alert contract.
//
// Author: monsterfei
// Date: 2026-07-03
// @param summary Persisted market summary.
// @returns Alert-shaped summary payload for notification senders.
func summaryAlert(summary model.MarketSummary) model.Alert {
	return model.Alert{
		ID:          summary.ID,
		Exchange:    "summary",
		Symbol:      "MARKET",
		Type:        "market_summary",
		Severity:    "info",
		Title:       "AI 市场摘要",
		Message:     summary.Content,
		TriggerTime: summary.WindowTo,
		CreatedAt:   summary.CreatedAt,
	}
}

// now returns the configured clock value or current UTC time.
//
// Author: monsterfei
// Date: 2026-06-30
// @returns Current time for this service run.
func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}
