package user

import (
	"context"

	"github.com/renfei198727/crypto-watchtower/internal/api"
	authsvc "github.com/renfei198727/crypto-watchtower/internal/auth"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// ProfileRepository defines bounded user profile lookups for the dashboard.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type ProfileRepository interface {
	FindByID(context.Context, int64) (model.User, bool, error)
}

// DeliveryPreferenceRepository defines Telegram delivery preference writes.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type DeliveryPreferenceRepository interface {
	UpdateTelegramDeliveryEnabled(context.Context, int64, bool) error
}

// AlertRepository defines bounded user alert history reads for the dashboard.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type AlertRepository interface {
	ListForUser(context.Context, int64, int) ([]model.Alert, error)
}

// NotificationRepository defines bounded user notification-log reads for profile state.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type NotificationRepository interface {
	LatestForUser(context.Context, int64, int) ([]model.NotificationLog, error)
}

// RuleCountRepository defines bounded user rule-count reads for subscription limits.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type RuleCountRepository interface {
	CountUserRules(context.Context, int64) (int64, error)
}

// Service coordinates user-facing dashboard reads.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// modified by __AUTHOR__ on 2026-07-01
type Service struct {
	profiles      ProfileRepository
	preferences   DeliveryPreferenceRepository
	alerts        AlertRepository
	notifications NotificationRepository
	rules         RuleCountRepository
}

// NewService creates the user dashboard service from narrow repositories.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param profiles Repository for bounded user profile reads.
// @param alerts Repository for bounded user alert history reads.
// @param rules Repository for bounded user rule-count reads.
// @param notifications Optional repository for bounded recent notification state.
// @returns User dashboard service.
// modified by __AUTHOR__ on 2026-07-01
func NewService(profiles ProfileRepository, alerts AlertRepository, rules RuleCountRepository, notifications ...NotificationRepository) Service {
	preferences, _ := profiles.(DeliveryPreferenceRepository)
	var notificationRepo NotificationRepository
	if len(notifications) > 0 {
		notificationRepo = notifications[0]
	}
	return Service{
		profiles:      profiles,
		preferences:   preferences,
		alerts:        alerts,
		notifications: notificationRepo,
		rules:         rules,
	}
}

// Profile returns safe user profile metadata for the dashboard.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id to look up.
// @returns Profile metadata with Telegram binding details masked.
func (s Service) Profile(ctx context.Context, userID int64) (api.UserProfile, error) {
	profile := api.UserProfile{UserID: userID}
	if s.profiles == nil {
		return profile, nil
	}
	user, ok, err := s.profiles.FindByID(ctx, userID)
	if err != nil {
		return api.UserProfile{}, err
	}
	if !ok {
		return profile, nil
	}
	plan := user.Plan
	if plan == "" {
		plan = model.UserPlanFree
	}
	status := user.Status
	if status == "" {
		status = model.UserStatusActive
	}
	profile.UserID = user.ID
	profile.TelegramBound = user.TelegramChatID != ""
	profile.TelegramChatIDMasked = maskBinding(user.TelegramChatID)
	profile.TelegramDeliveryEnabled = user.TelegramDeliveryEnabled
	if s.notifications != nil {
		logs, err := s.notifications.LatestForUser(ctx, userID, 1)
		if err != nil {
			return api.UserProfile{}, err
		}
		if len(logs) > 0 {
			profile.RecentDeliveryStatus = logs[0].Status
		}
	}
	profile.Plan = plan
	profile.Status = status
	profile.Limits = authsvc.LimitsForPlan(plan)
	return profile, nil
}

// UpdateTelegramDeliveryEnabled persists a Telegram delivery preference and returns the refreshed profile.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to update.
// @param enabled Whether Telegram user-rule delivery is enabled.
// @returns Refreshed profile metadata.
func (s Service) UpdateTelegramDeliveryEnabled(ctx context.Context, userID int64, enabled bool) (api.UserProfile, error) {
	if s.preferences != nil {
		if err := s.preferences.UpdateTelegramDeliveryEnabled(ctx, userID, enabled); err != nil {
			return api.UserProfile{}, err
		}
	}
	profile, err := s.Profile(ctx, userID)
	if err != nil {
		return api.UserProfile{}, err
	}
	profile.TelegramDeliveryEnabled = enabled
	return profile, nil
}

// ListAlerts returns alert history that belongs to one user.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id that owns notification history.
// @param limit Maximum number of alerts to return.
// @returns Bounded alert history for the user.
func (s Service) ListAlerts(ctx context.Context, userID int64, limit int) ([]model.Alert, error) {
	if s.alerts == nil {
		return []model.Alert{}, nil
	}
	return s.alerts.ListForUser(ctx, userID, limit)
}

// CanCreateRule checks whether a user can add another personal alert rule.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param user Current authenticated user.
// @returns Error when the user's plan or status blocks rule creation.
func (s Service) CanCreateRule(ctx context.Context, user model.User) error {
	if user.Status == model.UserStatusDisabled {
		return api.ErrUserDisabled
	}
	if s.rules == nil {
		return nil
	}
	count, err := s.rules.CountUserRules(ctx, user.ID)
	if err != nil {
		return err
	}
	limit := authsvc.LimitsForPlan(user.Plan).MaxRules
	if count >= int64(limit) {
		return api.ErrSubscriptionRuleLimitExceeded
	}
	return nil
}

// AlertHistoryLimit caps requested alert history to the user's subscription entitlement.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param user Current authenticated user.
// @param requested Requested alert history limit.
// @returns Bounded alert history limit.
func (s Service) AlertHistoryLimit(user model.User, requested int) int {
	limit := authsvc.LimitsForPlan(user.Plan).AlertHistory
	if requested <= 0 || requested > limit {
		return limit
	}
	return requested
}

// maskBinding keeps only the last four characters of a binding identifier.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param value Binding identifier to mask.
// @returns Masked binding identifier.
func maskBinding(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}
