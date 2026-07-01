package scheduler

import (
	"context"
	"testing"
	"time"
)

// TestUserDigestJobRunOnceFlushesDueDigests verifies digest jobs delegate one flush cycle.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestUserDigestJobRunOnceFlushesDueDigests(t *testing.T) {
	flusher := &fakeUserDigestFlusher{}
	job := NewUserDigestJob(flusher, time.Minute)
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	if err := job.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("run digest job: %v", err)
	}
	if !flusher.lastNow.Equal(now) {
		t.Fatalf("expected digest flusher time %s, got %s", now, flusher.lastNow)
	}
}

// fakeUserDigestFlusher records digest flush calls for scheduler tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type fakeUserDigestFlusher struct {
	lastNow time.Time
}

// FlushUserDigests records one digest flush request for scheduler tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param now Current time passed by the job.
// @returns Nil error for tests.
func (f *fakeUserDigestFlusher) FlushUserDigests(_ context.Context, now time.Time) error {
	f.lastNow = now
	return nil
}
