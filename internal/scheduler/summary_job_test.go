package scheduler

import (
	"context"
	"testing"
	"time"
)

// TestSummaryJobRunOnceDelegatesToRunner verifies manual runs call the summary runner.
//
// Author: monsterfei
// Date: 2026-06-30
func TestSummaryJobRunOnceDelegatesToRunner(t *testing.T) {
	runner := &fakeSummaryRunner{}
	job := NewSummaryJob(runner, time.Minute)

	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("run summary job: %v", err)
	}
	if runner.count != 1 {
		t.Fatalf("expected one run, got %d", runner.count)
	}
}

type fakeSummaryRunner struct {
	count int
}

// RunOnce records summary job calls for scheduler tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (f *fakeSummaryRunner) RunOnce(context.Context) error {
	f.count++
	return nil
}
