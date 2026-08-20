package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type stubUserService struct {
	profile                     UserProfile
	alerts                      []model.Alert
	notifications               []UserNotificationLog
	ruleErr                     error
	lastUserID                  int64
	lastLimit                   int
	lastDeliveryOn              bool
	lastNotificationPreferences model.UserNotificationPreferences
	unboundUserID               int64
}

// Profile returns one user profile for user API tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (s *stubUserService) Profile(_ context.Context, userID int64) (UserProfile, error) {
	s.lastUserID = userID
	return s.profile, nil
}

// ListAlerts returns one user's alert history for user API tests.
//
// Author: monsterfei
// Date: 2026-06-30
func (s *stubUserService) ListAlerts(_ context.Context, userID int64, limit int) ([]model.Alert, error) {
	s.lastUserID = userID
	s.lastLimit = limit
	return s.alerts, nil
}

// ListNotificationLogs returns one user's notification history for user API tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubUserService) ListNotificationLogs(_ context.Context, userID int64, limit int) ([]UserNotificationLog, error) {
	s.lastUserID = userID
	s.lastLimit = limit
	return s.notifications, nil
}

// CanCreateRule returns the configured rule-limit result for user API tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubUserService) CanCreateRule(context.Context, model.User) error {
	return s.ruleErr
}

// AlertHistoryLimit returns the requested limit for user API tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubUserService) AlertHistoryLimit(_ model.User, requested int) int {
	return requested
}

// UpdateTelegramDeliveryEnabled records one Telegram delivery preference update for user API tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubUserService) UpdateTelegramDeliveryEnabled(_ context.Context, userID int64, enabled bool) (UserProfile, error) {
	s.lastUserID = userID
	s.lastDeliveryOn = enabled
	s.profile.UserID = userID
	s.profile.TelegramDeliveryEnabled = enabled
	return s.profile, nil
}

// UpdateTelegramNotificationPreferences records one quiet-hours and digest preference update for user API tests.
//
// Author: monsterfei
// Date: 2026-07-01
// @param userID User id to update.
// @param preferences Quiet-hours and digest preference payload.
// @returns Updated profile payload and nil error.
func (s *stubUserService) UpdateTelegramNotificationPreferences(_ context.Context, userID int64, preferences model.UserNotificationPreferences) (UserProfile, error) {
	s.lastUserID = userID
	s.lastNotificationPreferences = preferences
	s.profile.UserID = userID
	return s.profile, nil
}

// UnbindTelegram records one Telegram unbind request for user API tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubUserService) UnbindTelegram(_ context.Context, userID int64) (UserProfile, error) {
	s.unboundUserID = userID
	s.profile.UserID = userID
	s.profile.TelegramBound = false
	s.profile.TelegramChatIDMasked = ""
	return s.profile, nil
}

// stubTelegramBindingService returns binding tokens for user API tests.
//
// Author: monsterfei
// Date: 2026-07-01
type stubTelegramBindingService struct {
	rawToken   string
	expiresAt  time.Time
	lastUserID int64
}

// CreateBindingToken returns a configured Telegram binding token for API tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubTelegramBindingService) CreateBindingToken(_ context.Context, userID int64) (string, time.Time, error) {
	s.lastUserID = userID
	if s.rawToken == "" {
		s.rawToken = "bind-token"
	}
	if s.expiresAt.IsZero() {
		s.expiresAt = time.Now().UTC().Add(time.Minute)
	}
	return s.rawToken, s.expiresAt, nil
}

// TestUserProfileRequiresSession verifies user profile APIs require a valid session.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserProfileRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/profile", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestUserRulesListUsesSessionUser verifies user rule reads are scoped to the session user.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserRulesListUsesSessionUser(t *testing.T) {
	userID := int64(42)
	ruleSvc := &stubRuleService{
		userRules: []model.AlertRule{{
			UserID:    &userID,
			Scope:     "user",
			Exchange:  "binance",
			Symbol:    "BTCUSDT",
			RuleType:  "large_trade",
			Threshold: 120000,
			Enabled:   true,
		}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/rules?user_id=99", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth:  &stubAuthService{currentUser: model.User{ID: userID, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		Rules: ruleSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if ruleSvc.lastUserID != 42 {
		t.Fatalf("expected user rule filter, got %d", ruleSvc.lastUserID)
	}
	var payload struct {
		Data []model.AlertRule `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Symbol != "BTCUSDT" {
		t.Fatalf("unexpected user rules payload: %+v", payload.Data)
	}
}

// TestUserRulesListReturnsEmptyArray verifies empty user rule reads stay list-shaped for dashboard consumers.
//
// Author: monsterfei
// Date: 2026-07-02
func TestUserRulesListReturnsEmptyArray(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/rules", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth:  &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		Rules: &stubRuleService{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if string(payload.Data) != "[]" {
		t.Fatalf("expected empty user rules array, got %s", payload.Data)
	}
}

// TestUserRulesPostUsesSessionUser verifies user rule writes use the session user as owner.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserRulesPostUsesSessionUser(t *testing.T) {
	ruleSvc := &stubRuleService{}
	body := []byte(`{"user_id":99,"exchange":"okx","symbol":"BTCUSDT","rule_type":"large_trade","threshold":130000,"window_sec":90,"enabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/rules", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth:  &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		Rules: ruleSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if ruleSvc.userRule == nil || ruleSvc.userRule.UserID == nil || *ruleSvc.userRule.UserID != 42 {
		t.Fatalf("expected user rule to be written, got %+v", ruleSvc.userRule)
	}
	if ruleSvc.userRule.Exchange != "okx" || ruleSvc.userRule.WindowSec != 90 {
		t.Fatalf("unexpected user rule payload: %+v", ruleSvc.userRule)
	}
}

// TestUserProfileUsesSessionUser verifies profile reads expose masked binding state for the session user.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserProfileUsesSessionUser(t *testing.T) {
	userSvc := &stubUserService{
		profile: UserProfile{
			UserID:                  42,
			TelegramBound:           true,
			TelegramChatIDMasked:    "****7890",
			TelegramDeliveryEnabled: true,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/profile?user_id=99", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth: &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		User: userSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data UserProfile `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if userSvc.lastUserID != 42 {
		t.Fatalf("expected user id 42, got %d", userSvc.lastUserID)
	}
	if !payload.Data.TelegramBound || payload.Data.TelegramChatIDMasked != "****7890" {
		t.Fatalf("unexpected profile payload: %+v", payload.Data)
	}
	if !payload.Data.TelegramDeliveryEnabled {
		t.Fatalf("expected telegram delivery enabled in profile: %+v", payload.Data)
	}
}

// TestUserAlertsListUsesSessionUser verifies alert history is scoped to the session user.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserAlertsListUsesSessionUser(t *testing.T) {
	userSvc := &stubUserService{
		alerts: []model.Alert{{
			ID:       "alert-1",
			Exchange: "binance",
			Symbol:   "BTCUSDT",
			Type:     "large_trade",
			Title:    "Large trade",
		}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/alerts?user_id=99&limit=5", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth: &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		User: userSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if userSvc.lastUserID != 42 || userSvc.lastLimit != 5 {
		t.Fatalf("unexpected alert query: user_id=%d limit=%d", userSvc.lastUserID, userSvc.lastLimit)
	}
	var payload struct {
		Data []model.Alert `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "alert-1" {
		t.Fatalf("unexpected alert payload: %+v", payload.Data)
	}
}

// TestUserNotificationsListRequiresSession verifies notification history requires login.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserNotificationsListRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/notifications", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}, User: &stubUserService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestUserNotificationsListUsesSessionUser verifies notification history ignores query user ids.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserNotificationsListUsesSessionUser(t *testing.T) {
	userSvc := &stubUserService{
		notifications: []UserNotificationLog{{
			AlertID:      "alert-1",
			Channel:      "telegram",
			Target:       "****7890",
			Status:       "failed",
			ErrorMessage: "send failed",
			CreatedAt:    time.Unix(1710000002, 0).UTC(),
		}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/notifications?user_id=99&limit=3", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth: &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		User: userSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if userSvc.lastUserID != 42 || userSvc.lastLimit != 3 {
		t.Fatalf("unexpected notification query: user_id=%d limit=%d", userSvc.lastUserID, userSvc.lastLimit)
	}
	var payload struct {
		Data []UserNotificationLog `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Target != "****7890" || payload.Data[0].Status != "failed" {
		t.Fatalf("unexpected notification payload: %+v", payload.Data)
	}
}

// TestTelegramNotificationPreferencesRequiresSession verifies quiet-hours and digest preference APIs require login.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramNotificationPreferencesRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/telegram/preferences", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}, User: &stubUserService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestTelegramNotificationPreferencesUsesSessionUser verifies quiet-hours and digest writes ignore external user ids.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramNotificationPreferencesUsesSessionUser(t *testing.T) {
	userSvc := &stubUserService{profile: UserProfile{UserID: 42, TelegramDeliveryEnabled: true}}
	body := []byte(`{
		"user_id": 99,
		"telegram_quiet_hours_enabled": true,
		"telegram_quiet_hours_start": "23:30",
		"telegram_quiet_hours_end": "07:15",
		"telegram_quiet_hours_timezone": "Asia/Shanghai",
		"telegram_digest_enabled": true,
		"telegram_digest_interval_min": 45
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/telegram/preferences?user_id=99", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth: &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		User: userSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if userSvc.lastUserID != 42 {
		t.Fatalf("expected session user id 42, got %d", userSvc.lastUserID)
	}
	got := userSvc.lastNotificationPreferences
	if !got.TelegramQuietHoursEnabled || got.TelegramQuietHoursStart != "23:30" || got.TelegramQuietHoursEnd != "07:15" || got.TelegramQuietHoursTimezone != "Asia/Shanghai" {
		t.Fatalf("unexpected quiet-hours preferences: %+v", got)
	}
	if !got.TelegramDigestEnabled || got.TelegramDigestIntervalMin != 45 {
		t.Fatalf("unexpected digest preferences: %+v", got)
	}
}

// TestTelegramBindingTokenRequiresSession verifies binding-token requests require login.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramBindingTokenRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/telegram/binding-token", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}, TelegramBinding: &stubTelegramBindingService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestTelegramBindingTokenUsesSessionUser verifies binding tokens are created for the current user.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramBindingTokenUsesSessionUser(t *testing.T) {
	binding := &stubTelegramBindingService{rawToken: "bind-token", expiresAt: time.Unix(1710000000, 0).UTC()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/telegram/binding-token", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth:            &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		TelegramBinding: binding,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if binding.lastUserID != 42 {
		t.Fatalf("expected binding token for user 42, got %d", binding.lastUserID)
	}
	var payload struct {
		Data struct {
			Token     string    `json:"token"`
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.Token != "bind-token" || !payload.Data.ExpiresAt.Equal(binding.expiresAt) {
		t.Fatalf("unexpected binding payload: %+v", payload.Data)
	}
}

// TestTelegramDeliveryPreferenceRequiresSession verifies delivery preference writes require login.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramDeliveryPreferenceRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/telegram/delivery", strings.NewReader(`{"enabled":false}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}, User: &stubUserService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestTelegramDeliveryPreferenceUsesSessionUser verifies delivery preferences update the current user only.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramDeliveryPreferenceUsesSessionUser(t *testing.T) {
	userSvc := &stubUserService{profile: UserProfile{TelegramBound: true, TelegramChatIDMasked: "****2345"}}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/telegram/delivery?user_id=99", strings.NewReader(`{"enabled":false}`))
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth: &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		User: userSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if userSvc.lastUserID != 42 || userSvc.lastDeliveryOn {
		t.Fatalf("expected delivery update for user 42 false, got user_id=%d enabled=%v", userSvc.lastUserID, userSvc.lastDeliveryOn)
	}
	var payload struct {
		Data UserProfile `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.UserID != 42 || payload.Data.TelegramDeliveryEnabled {
		t.Fatalf("unexpected delivery payload: %+v", payload.Data)
	}
}

// TestTelegramUnbindRequiresSession verifies Telegram unbind requires login.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramUnbindRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/telegram/binding", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}, User: &stubUserService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestTelegramUnbindUsesSessionUser verifies Telegram unbind ignores query user ids.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramUnbindUsesSessionUser(t *testing.T) {
	userSvc := &stubUserService{profile: UserProfile{TelegramDeliveryEnabled: true}}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/telegram/binding?user_id=99", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		Auth: &stubAuthService{currentUser: model.User{ID: 42, Status: model.UserStatusActive, Plan: model.UserPlanFree}, currentOK: true},
		User: userSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if userSvc.unboundUserID != 42 {
		t.Fatalf("expected unbind for user 42, got %d", userSvc.unboundUserID)
	}
	var payload struct {
		Data UserProfile `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.TelegramBound || payload.Data.TelegramChatIDMasked != "" || !payload.Data.TelegramDeliveryEnabled {
		t.Fatalf("unexpected unbind payload: %+v", payload.Data)
	}
}

// TestDashboardPageIsServed verifies the user dashboard page is separate from admin.
//
// Author: monsterfei
// Date: 2026-06-30
func TestDashboardPageIsServed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		"CryptoWatchtower Dashboard",
		`id="login-email"`,
		`id="login-password"`,
		`id="register-email"`,
		`id="register-password"`,
		`id="logout-button"`,
		`id="telegram-bind-button"`,
		`id="telegram-binding-token"`,
		`id="telegram-delivery-enabled"`,
		`id="telegram-delivery-status"`,
		`id="telegram-preferences-form"`,
		`id="telegram-quiet-hours-enabled"`,
		`id="telegram-quiet-hours-start"`,
		`id="telegram-quiet-hours-end"`,
		`id="telegram-quiet-hours-timezone"`,
		`id="telegram-digest-enabled"`,
		`id="telegram-digest-interval"`,
		`id="telegram-unbind-button"`,
		`id="notifications-table"`,
		"Subscription",
		"Telegram Binding",
		"Notification Logs",
		"Personal Rules",
		"Alert History",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in dashboard page, got %s", expected, body)
		}
	}
	for _, removed := range []string{`id="bearer-token"`, `id="user-id"`} {
		if strings.Contains(body, removed) {
			t.Fatalf("did not expect legacy control %q in dashboard page", removed)
		}
	}
}
