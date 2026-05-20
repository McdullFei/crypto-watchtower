package scheduler

import (
	"context"
	"time"
)

type Fetcher interface {
	Fetch(context.Context) error
}

type FundingJob struct {
	fetcher  Fetcher
	interval time.Duration
}

func NewFundingJob(fetcher Fetcher, interval time.Duration) FundingJob {
	return FundingJob{fetcher: fetcher, interval: interval}
}

func (j FundingJob) Start(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = j.fetcher.Fetch(ctx)
		}
	}
}
