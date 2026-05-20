package storage

import "github.com/jackc/pgx/v5/pgxpool"

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
