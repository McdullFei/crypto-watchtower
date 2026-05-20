package scheduler

import (
	"context"
	"testing"
	"time"
)

type stubFetcher struct{ called bool }

func (s *stubFetcher) Fetch(context.Context) error {
	s.called = true
	return nil
}

func TestFundingJobCallsFetcher(t *testing.T) {
	fetcher := &stubFetcher{}
	job := NewFundingJob(fetcher, time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go job.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	if !fetcher.called {
		t.Fatal("expected fetcher to be called")
	}
}
