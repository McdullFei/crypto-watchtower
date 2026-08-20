package user

import (
	"context"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/api"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type stubProfileRepository struct {
	user model.User
	ok   bool
}

type stubAlertRepository struct {
	alerts     []model.Alert
	lastUserID int64
	lastLimit  int
}

// stubNotificationRepository returns recent notification logs for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
type stubNotificationRepository struct {
	logs       []model.NotificationLog
	lastUserID int64
	lastLimit  int
}

// stubRuleCountRepository returns a configured rule count for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
type stubRuleCountRepository struct {
	count int64
}

// FindByID returns one user for service tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (s stubProfileRepository) FindByID(context.Context, int64) (model.User, bool, error) {
	return s.user, s.ok, nil
}

// ListForUser returns alert history for service tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (s *stubAlertRepository) ListForUser(_ context.Context, userID int64, limit int) ([]model.Alert, error) {
	s.lastUserID = userID
	s.lastLimit = limit
	return s.alerts, nil
}

// LatestForUser returns recent notification logs for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubNotificationRepository) LatestForUser(_ context.Context, userID int64, limit int) ([]model.NotificationLog, error) {
	s.lastUserID = userID
	s.lastLimit = limit
	return s.logs, nil
}

// CountUserRules returns the configured user rule count for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s stubRuleCountRepository) CountUserRules(context.Context, int64) (int64, error) {
	return s.count, nil
}

// TestServiceProfileMasksTelegramBinding verifies Telegram metadata is not exposed raw.
//
// Author: monsterfei
// Date: 2026-06-30
func TestServiceProfileMasksTelegramBinding(t *testing.T) {
	service := NewService(stubProfileRepository{
		user: model.User{
			ID:                      42,
			TelegramChatID:          "1234567890",
			TelegramDeliveryEnabled: true,
			CreatedAt:               time.Unix(1710000000, 0).UTC(),
			UpdatedAt:               time.Unix(1710000001, 0).UTC(),
		},
		ok: true,
	}, nil, nil)

	profile, err := service.Profile(context.Background(), 42)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.UserID != 42 || !profile.TelegramBound || profile.TelegramChatIDMasked != "****7890" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

// TestServiceProfileIncludesRecentDeliveryStatus verifies profile reads include latest user notification state.
//
// Author: monsterfei
// Date: 2026-07-01
func TestServiceProfileIncludesRecentDeliveryStatus(t *testing.T) {
	service := NewService(stubProfileRepository{
		user: model.User{
			ID:                      42,
			TelegramChatID:          "1234567890",
			TelegramDeliveryEnabled: true,
		},
		ok: true,
	}, nil, nil, &stubNotificationRepository{logs: []model.NotificationLog{{Status: "failed"}}})

	profile, err := service.Profile(context.Background(), 42)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.RecentDeliveryStatus != "failed" {
		t.Fatalf("expected recent delivery status failed, got %+v", profile)
	}
}

// TestServiceListNotificationLogsMasksTargets verifies user notification logs are scoped and masked.
//
// Author: monsterfei
// Date: 2026-07-01
func TestServiceListNotificationLogsMasksTargets(t *testing.T) {
	userID := int64(42)
	notifications := &stubNotificationRepository{
		logs: []model.NotificationLog{{
			UserID:       &userID,
			AlertID:      "alert-1",
			Channel:      "telegram",
			Target:       "1234567890",
			Status:       "sent",
			ErrorMessage: "",
			CreatedAt:    time.Unix(1710000002, 0).UTC(),
		}},
	}
	service := NewService(nil, nil, nil, notifications)

	logs, err := service.ListNotificationLogs(context.Background(), userID, 5)
	if err != nil {
		t.Fatalf("list notification logs: %v", err)
	}
	if notifications.lastUserID != userID || notifications.lastLimit != 5 {
		t.Fatalf("unexpected notification query: user_id=%d limit=%d", notifications.lastUserID, notifications.lastLimit)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one notification log, got %+v", logs)
	}
	if logs[0].Target != "****7890" || logs[0].Target == "1234567890" {
		t.Fatalf("expected masked target, got %+v", logs[0])
	}
	if logs[0].Channel != "telegram" || logs[0].AlertID != "alert-1" || logs[0].Status != "sent" {
		t.Fatalf("unexpected notification log payload: %+v", logs[0])
	}
}

// TestServiceListAlertsUsesUserScope verifies alert history is read through user ownership.
//
// Author: monsterfei
// Date: 2026-06-30
func TestServiceListAlertsUsesUserScope(t *testing.T) {
	alerts := &stubAlertRepository{
		alerts: []model.Alert{{ID: "alert-1", Symbol: "BTCUSDT"}},
	}
	service := NewService(nil, alerts, nil)

	got, err := service.ListAlerts(context.Background(), 42, 5)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if alerts.lastUserID != 42 || alerts.lastLimit != 5 {
		t.Fatalf("unexpected alert query: user_id=%d limit=%d", alerts.lastUserID, alerts.lastLimit)
	}
	if len(got) != 1 || got[0].ID != "alert-1" {
		t.Fatalf("unexpected alerts: %+v", got)
	}
}

// TestServiceCanCreateRuleEnforcesPlanLimit verifies subscription rule-count limits are enforced.
//
// Author: monsterfei
// Date: 2026-07-01
func TestServiceCanCreateRuleEnforcesPlanLimit(t *testing.T) {
	service := NewService(nil, nil, stubRuleCountRepository{count: 5})

	err := service.CanCreateRule(context.Background(), model.User{ID: 42, Plan: model.UserPlanFree, Status: model.UserStatusActive})

	if err != api.ErrSubscriptionRuleLimitExceeded {
		t.Fatalf("expected rule limit error, got %v", err)
	}
}

// TestServiceAlertHistoryLimitCapsRequestedLimit verifies alert history requests are plan bounded.
//
// Author: monsterfei
// Date: 2026-07-01
func TestServiceAlertHistoryLimitCapsRequestedLimit(t *testing.T) {
	service := NewService(nil, nil, nil)

	limit := service.AlertHistoryLimit(model.User{Plan: model.UserPlanFree}, 100)

	if limit != 20 {
		t.Fatalf("expected free history limit 20, got %d", limit)
	}
}
