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
