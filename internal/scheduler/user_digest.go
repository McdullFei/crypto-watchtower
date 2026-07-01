package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// UserDigestFlusher flushes due user digest notifications.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type UserDigestFlusher interface {
	FlushUserDigests(context.Context, time.Time) error
}

// UserDigestJob periodically sends due user digest notifications.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type UserDigestJob struct {
	flusher  UserDigestFlusher
	interval time.Duration
}

// NewUserDigestJob creates a periodic user digest flush job.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param flusher Digest flusher.
// @param interval Flush interval.
// @returns User digest job instance.
func NewUserDigestJob(flusher UserDigestFlusher, interval time.Duration) UserDigestJob {
	return UserDigestJob{flusher: flusher, interval: interval}
}

// RunOnce flushes digest notifications that are due at the provided time.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param now Current time for due-window evaluation.
// @returns Error returned by the flusher.
func (j UserDigestJob) RunOnce(ctx context.Context, now time.Time) error {
	return j.flusher.FlushUserDigests(ctx, now)
}

// Start runs digest flushing periodically until the context is canceled.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
func (j UserDigestJob) Start(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := j.RunOnce(ctx, time.Now().UTC()); err != nil {
				slog.Error("run user digest job", "err", err)
			}
		}
	}
}
