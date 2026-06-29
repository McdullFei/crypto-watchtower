package admin

import (
	"context"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/api"
	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/storage"
)

type Service struct {
	repos *storage.Repositories
}

func NewService(repos *storage.Repositories) Service {
	return Service{repos: repos}
}

func (s Service) Overview(ctx context.Context) (api.AdminOverview, error) {
	since := time.Now().UTC().Add(-24 * time.Hour)

	ruleCount, err := s.repos.AlertRules.CountEnabled(ctx)
	if err != nil {
		return api.AdminOverview{}, err
	}
	alertCount, err := s.repos.Alerts.CountSince(ctx, since)
	if err != nil {
		return api.AdminOverview{}, err
	}
	eventCount, err := s.repos.MarketEvents.CountSince(ctx, since)
	if err != nil {
		return api.AdminOverview{}, err
	}
	notificationCount, err := s.repos.NotificationLogs.CountSince(ctx, since)
	if err != nil {
		return api.AdminOverview{}, err
	}
	lastAlertAt, err := s.repos.Alerts.LatestCreatedAt(ctx)
	if err != nil {
		return api.AdminOverview{}, err
	}

	return api.AdminOverview{
		RuleCount:         ruleCount,
		AlertCount24h:     alertCount,
		EventCount24h:     eventCount,
		NotificationCount: notificationCount,
		LastAlertAt:       lastAlertAt,
	}, nil
}

// Trends returns compact 24-hour alert and notification aggregates for operators.
//
// Author: monsterfei
// Date: 2026-06-29
func (s Service) Trends(ctx context.Context) (api.AdminTrends, error) {
	since := time.Now().UTC().Add(-24 * time.Hour)

	alertCount, err := s.repos.Alerts.CountSince(ctx, since)
	if err != nil {
		return api.AdminTrends{}, err
	}
	statusCounts, err := s.repos.NotificationLogs.CountByStatusSince(ctx, since)
	if err != nil {
		return api.AdminTrends{}, err
	}
	symbolCounts, err := s.repos.Alerts.CountBySymbolSince(ctx, since, 8)
	if err != nil {
		return api.AdminTrends{}, err
	}

	out := api.AdminTrends{
		Alerts24h: alertCount,
		Notifications24h: api.AdminNotificationTrend{
			Sent:   statusCounts["sent"],
			Failed: statusCounts["failed"],
		},
		SymbolAlerts24h: make([]api.AdminSymbolCount, 0, len(symbolCounts)),
	}
	for _, item := range symbolCounts {
		out.SymbolAlerts24h = append(out.SymbolAlerts24h, api.AdminSymbolCount{
			Symbol: item.Symbol,
			Count:  item.Count,
		})
	}
	return out, nil
}

func (s Service) ListRules(ctx context.Context, filter api.AdminListFilter) ([]model.AlertRule, error) {
	return s.repos.AlertRules.List(ctx, storage.ListFilter{
		Exchange: filter.Exchange,
		Symbol:   filter.Symbol,
		RuleType: filter.RuleType,
		Limit:    filter.Limit,
	})
}

func (s Service) ListAlerts(ctx context.Context, filter api.AdminListFilter) ([]model.Alert, error) {
	return s.repos.Alerts.List(ctx, storage.ListFilter{
		Exchange: filter.Exchange,
		Symbol:   filter.Symbol,
		RuleType: filter.RuleType,
		Limit:    filter.Limit,
	})
}

func (s Service) ListEvents(ctx context.Context, filter api.AdminListFilter) ([]model.MarketEvent, error) {
	return s.repos.MarketEvents.List(ctx, storage.ListFilter{
		Exchange:  filter.Exchange,
		Symbol:    filter.Symbol,
		EventType: filter.EventType,
		Limit:     filter.Limit,
	})
}

func (s Service) ListNotifications(ctx context.Context, filter api.AdminListFilter) ([]model.NotificationLog, error) {
	return s.repos.NotificationLogs.List(ctx, storage.ListFilter{
		Status: filter.Status,
		Limit:  filter.Limit,
	})
}
