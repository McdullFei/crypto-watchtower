package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type Repositories struct {
	MarketEvents     MarketEventRepo
	AlertRules       AlertRuleRepo
	Alerts           AlertRepo
	NotificationLogs NotificationLogRepo
	Users            UserRepo
}

func NewRepositories(db *pgxpool.Pool) *Repositories {
	return &Repositories{
		MarketEvents:     MarketEventRepo{DB: db},
		AlertRules:       AlertRuleRepo{DB: db},
		Alerts:           AlertRepo{DB: db},
		NotificationLogs: NotificationLogRepo{DB: db},
		Users:            UserRepo{DB: db},
	}
}

// InsertMarketEvent stores one alert-related market event through the repository set.
//
// Author: __AUTHOR__
// Date: 2026-06-29
// @param ctx Request context.
// @param event Market event to persist.
// @returns Error when persistence fails.
func (r *Repositories) InsertMarketEvent(ctx context.Context, event model.MarketEvent) error {
	return r.MarketEvents.Insert(ctx, event)
}

// InsertAlert stores one generated alert through the repository set.
//
// Author: __AUTHOR__
// Date: 2026-06-29
// @param ctx Request context.
// @param alert Alert to persist.
// @returns Error when persistence fails.
func (r *Repositories) InsertAlert(ctx context.Context, alert model.Alert) error {
	return r.Alerts.Insert(ctx, alert)
}

// InsertNotificationLog stores one notification delivery result through the repository set.
//
// Author: __AUTHOR__
// Date: 2026-06-29
// @param ctx Request context.
// @param log Notification log to persist.
// @returns Error when persistence fails.
func (r *Repositories) InsertNotificationLog(ctx context.Context, log model.NotificationLog) error {
	return r.NotificationLogs.Insert(ctx, log)
}
