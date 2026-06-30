package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// SummaryRunner runs one market summary cycle.
//
// Author: monsterfei
// Date: 2026-06-30
type SummaryRunner interface {
	RunOnce(context.Context) error
}

// SummaryJob periodically runs the AI market summary service.
//
// Author: monsterfei
// Date: 2026-06-30
type SummaryJob struct {
	runner   SummaryRunner
	interval time.Duration
}

// NewSummaryJob creates a periodic summary job.
//
// Author: monsterfei
// Date: 2026-06-30
// @param runner Summary runner.
// @param interval Run interval.
// @returns Summary job instance.
func NewSummaryJob(runner SummaryRunner, interval time.Duration) SummaryJob {
	return SummaryJob{runner: runner, interval: interval}
}

// RunOnce delegates a single summary run to the runner.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @returns Error returned by the runner.
func (j SummaryJob) RunOnce(ctx context.Context) error {
	return j.runner.RunOnce(ctx)
}

// Start runs summaries periodically until the context is canceled.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
func (j SummaryJob) Start(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := j.RunOnce(ctx); err != nil {
				slog.Error("run summary job", "err", err)
			}
		}
	}
}
