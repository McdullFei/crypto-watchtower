package summary

import (
	"context"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/storage"
)

const defaultMaxItems = 50

// Config contains bounded snapshot aggregation settings.
//
// Author: monsterfei
// Date: 2026-06-30
type Config struct {
	MaxItems int
}

// Snapshot contains the persisted data used to generate one market summary.
//
// Author: monsterfei
// Date: 2026-06-30
type Snapshot struct {
	WindowFrom   time.Time
	WindowTo     time.Time
	AlertCount   int
	EventCount   int
	FundingCount int
	Alerts       []model.Alert
	Events       []model.MarketEvent
}

// AlertLister reads alerts for summary aggregation.
//
// Author: monsterfei
// Date: 2026-06-30
type AlertLister interface {
	List(context.Context, storage.ListFilter) ([]model.Alert, error)
}

// EventLister reads market events for summary aggregation.
//
// Author: monsterfei
// Date: 2026-06-30
type EventLister interface {
	List(context.Context, storage.ListFilter) ([]model.MarketEvent, error)
}

// Aggregator builds bounded snapshots from persisted alerts and events.
//
// Author: monsterfei
// Date: 2026-06-30
type Aggregator struct {
	alerts AlertLister
	events EventLister
	cfg    Config
}

// NewAggregator creates a bounded snapshot aggregator.
//
// Author: monsterfei
// Date: 2026-06-30
// @param alerts Alert lister.
// @param events Market event lister.
// @param cfg Aggregation settings.
// @returns Aggregator instance.
func NewAggregator(alerts AlertLister, events EventLister, cfg Config) Aggregator {
	return Aggregator{alerts: alerts, events: events, cfg: cfg}
}

// Build creates one bounded snapshot for the requested time window.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param from Inclusive window start.
// @param to Window end.
// @returns Snapshot containing bounded samples and fetched counts.
func (a Aggregator) Build(ctx context.Context, from time.Time, to time.Time) (Snapshot, error) {
	maxItems := a.cfg.MaxItems
	if maxItems <= 0 {
		maxItems = defaultMaxItems
	}
	limit := maxItems + 1

	alerts, err := a.alerts.List(ctx, storage.ListFilter{Since: from, Limit: limit})
	if err != nil {
		return Snapshot{}, err
	}
	events, err := a.events.List(ctx, storage.ListFilter{Since: from, Limit: limit})
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{
		WindowFrom:   from,
		WindowTo:     to,
		AlertCount:   len(alerts),
		EventCount:   len(events),
		FundingCount: countFundingEvents(events),
		Alerts:       capAlerts(alerts, maxItems),
		Events:       capEvents(events, maxItems),
	}
	return snapshot, nil
}

// countFundingEvents counts funding events in a bounded event sample.
//
// Author: monsterfei
// Date: 2026-06-30
// @param events Market events to inspect.
// @returns Number of funding events.
func countFundingEvents(events []model.MarketEvent) int {
	count := 0
	for _, event := range events {
		if event.EventType == "funding" {
			count++
		}
	}
	return count
}

// capAlerts limits alert samples to maxItems.
//
// Author: monsterfei
// Date: 2026-06-30
// @param alerts Alert rows to cap.
// @param maxItems Maximum returned sample size.
// @returns Bounded alert sample.
func capAlerts(alerts []model.Alert, maxItems int) []model.Alert {
	if len(alerts) <= maxItems {
		return alerts
	}
	return alerts[:maxItems]
}

// capEvents limits market event samples to maxItems.
//
// Author: monsterfei
// Date: 2026-06-30
// @param events Market event rows to cap.
// @param maxItems Maximum returned sample size.
// @returns Bounded market event sample.
func capEvents(events []model.MarketEvent, maxItems int) []model.MarketEvent {
	if len(events) <= maxItems {
		return events
	}
	return events[:maxItems]
}
