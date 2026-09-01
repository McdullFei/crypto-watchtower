package user

import (
	"context"

	"github.com/renfei198727/crypto-watchtower/internal/api"
	authsvc "github.com/renfei198727/crypto-watchtower/internal/auth"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// ProfileRepository defines bounded user profile lookups for the dashboard.
//
// Author: monsterfei
// Date: 2026-06-30
type ProfileRepository interface {
	FindByID(context.Context, int64) (model.User, bool, error)
}

// DeliveryPreferenceRepository defines Telegram delivery preference writes.
//
// Author: monsterfei
// Date: 2026-07-01
type DeliveryPreferenceRepository interface {
	UpdateTelegramDeliveryEnabled(context.Context, int64, bool) error
}

// NotificationPreferenceRepository defines Telegram quiet-hours and digest preference writes.
//
// Author: monsterfei
// Date: 2026-07-01
type NotificationPreferenceRepository interface {
	UpdateTelegramNotificationPreferences(context.Context, int64, model.UserNotificationPreferences) error
}

// TelegramUnbindRepository defines current-user Telegram unbind writes.
//
// Author: monsterfei
// Date: 2026-07-01
type TelegramUnbindRepository interface {
	UnbindTelegramChat(context.Context, int64) error
}

// AlertRepository defines bounded user alert history reads for the dashboard.
//
// Author: monsterfei
// Date: 2026-06-30
type AlertRepository interface {
	ListForUser(context.Context, int64, int) ([]model.Alert, error)
}

// NotificationRepository defines bounded user notification-log reads for profile state.
//
// Author: monsterfei
// Date: 2026-07-01
type NotificationRepository interface {
	LatestForUser(context.Context, int64, int) ([]model.NotificationLog, error)
}

// RuleCountRepository defines bounded user rule-count reads for subscription limits.
//
// Author: monsterfei
// Date: 2026-07-01
type RuleCountRepository interface {
	CountUserRules(context.Context, int64) (int64, error)
}

// Service coordinates user-facing dashboard reads.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-07-01
type Service struct {
	profiles                 ProfileRepository
	preferences              DeliveryPreferenceRepository
	notificationsPreferences NotificationPreferenceRepository
	unbinder                 TelegramUnbindRepository
	alerts                   AlertRepository
	notifications            NotificationRepository
	rules                    RuleCountRepository
}

// NewService creates the user dashboard service from narrow repositories.
//
// Author: monsterfei
// Date: 2026-06-30
// @param profiles Repository for bounded user profile reads.
// @param alerts Repository for bounded user alert history reads.
// @param rules Repository for bounded user rule-count reads.
// @param notifications Optional repository for bounded recent notification state.
// @returns User dashboard service.
// modified by monsterfei on 2026-07-01
func NewService(profiles ProfileRepository, alerts AlertRepository, rules RuleCountRepository, notifications ...NotificationRepository) Service {
	preferences, _ := profiles.(DeliveryPreferenceRepository)
	notificationPreferences, _ := profiles.(NotificationPreferenceRepository)
	unbinder, _ := profiles.(TelegramUnbindRepository)
	var notificationRepo NotificationRepository
	if len(notifications) > 0 {
		notificationRepo = notifications[0]
	}
	return Service{
		profiles:                 profiles,
		preferences:              preferences,
		notificationsPreferences: notificationPreferences,
		unbinder:                 unbinder,
		alerts:                   alerts,
		notifications:            notificationRepo,
		rules:                    rules,
	}
}

// Profile returns safe user profile metadata for the dashboard.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id to look up.
// @returns Profile metadata with Telegram binding details masked.
// modified by monsterfei on 2026-08-31
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
	profile.Email = user.Email
	profile.TelegramBound = user.TelegramChatID != ""
	profile.TelegramChatIDMasked = maskBinding(user.TelegramChatID)
	profile.TelegramDeliveryEnabled = user.TelegramDeliveryEnabled
	profile.NotificationPreferences = notificationPreferencesFromUser(user)
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
// Author: monsterfei
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

// UpdateTelegramNotificationPreferences persists quiet-hours and digest preferences and returns the refreshed profile.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to update.
// @param preferences Quiet-hours and digest preference values.
// @returns Refreshed profile metadata.
func (s Service) UpdateTelegramNotificationPreferences(ctx context.Context, userID int64, preferences model.UserNotificationPreferences) (api.UserProfile, error) {
	if s.notificationsPreferences != nil {
		if err := s.notificationsPreferences.UpdateTelegramNotificationPreferences(ctx, userID, preferences); err != nil {
			return api.UserProfile{}, err
		}
	}
	profile, err := s.Profile(ctx, userID)
	if err != nil {
		return api.UserProfile{}, err
	}
	profile.NotificationPreferences = preferences
	return profile, nil
}

// UnbindTelegram clears Telegram binding for one user and returns the refreshed profile.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to unbind.
// @returns Refreshed profile metadata.
func (s Service) UnbindTelegram(ctx context.Context, userID int64) (api.UserProfile, error) {
	if s.unbinder != nil {
		if err := s.unbinder.UnbindTelegramChat(ctx, userID); err != nil {
			return api.UserProfile{}, err
		}
	}
	return s.Profile(ctx, userID)
}

// ListAlerts returns alert history that belongs to one user.
//
// Author: monsterfei
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

// ListNotificationLogs returns safe bounded notification logs for one user.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id that owns notification history.
// @param limit Maximum number of logs to return.
// @returns Bounded notification logs with masked targets.
func (s Service) ListNotificationLogs(ctx context.Context, userID int64, limit int) ([]api.UserNotificationLog, error) {
	if s.notifications == nil {
		return []api.UserNotificationLog{}, nil
	}
	logs, err := s.notifications.LatestForUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]api.UserNotificationLog, 0, len(logs))
	for _, item := range logs {
		out = append(out, api.UserNotificationLog{
			AlertID:      item.AlertID,
			Channel:      item.Channel,
			Target:       maskBinding(item.Target),
			Status:       item.Status,
			ErrorMessage: item.ErrorMessage,
			CreatedAt:    item.CreatedAt,
		})
	}
	return out, nil
}

// CanCreateRule checks whether a user can add another personal alert rule.
//
// Author: monsterfei
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
// Author: monsterfei
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
// Author: monsterfei
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

// notificationPreferencesFromUser returns user preference fields with safe defaults.
//
// Author: monsterfei
// Date: 2026-07-01
// @param user User model containing persisted preference fields.
// @returns Telegram quiet-hours and digest preferences for API responses.
func notificationPreferencesFromUser(user model.User) model.UserNotificationPreferences {
	preferences := model.UserNotificationPreferences{
		TelegramQuietHoursEnabled:  user.TelegramQuietHoursEnabled,
		TelegramQuietHoursStart:    user.TelegramQuietHoursStart,
		TelegramQuietHoursEnd:      user.TelegramQuietHoursEnd,
		TelegramQuietHoursTimezone: user.TelegramQuietHoursTimezone,
		TelegramDigestEnabled:      user.TelegramDigestEnabled,
		TelegramDigestIntervalMin:  user.TelegramDigestIntervalMin,
	}
	if preferences.TelegramQuietHoursStart == "" {
		preferences.TelegramQuietHoursStart = "22:00"
	}
	if preferences.TelegramQuietHoursEnd == "" {
		preferences.TelegramQuietHoursEnd = "08:00"
	}
	if preferences.TelegramQuietHoursTimezone == "" {
		preferences.TelegramQuietHoursTimezone = "UTC"
	}
	if preferences.TelegramDigestIntervalMin <= 0 {
		preferences.TelegramDigestIntervalMin = 60
	}
	return preferences
}
