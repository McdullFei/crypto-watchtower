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

// Service orchestrates one bounded summary generation run.
//
// Author: monsterfei
// Date: 2026-06-30
type Service struct {
	Aggregator Aggregator
	Generator  Generator
	Store      SummaryStore
	Provider   string
	Window     time.Duration
	Now        func() time.Time
}

// RunOnce builds, generates, and stores one market summary.
//
// Author: monsterfei
// Date: 2026-06-30
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
	return nil
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
