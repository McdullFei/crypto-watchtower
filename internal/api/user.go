package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
	_ "time/tzdata"

	authsvc "github.com/renfei198727/crypto-watchtower/internal/auth"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

var (
	// ErrSubscriptionRuleLimitExceeded reports that a user has reached the plan rule limit.
	//
	// Author: monsterfei
	// Date: 2026-07-01
	ErrSubscriptionRuleLimitExceeded = errors.New("subscription rule limit exceeded")

	// ErrUserDisabled reports that a disabled user attempted a dashboard action.
	//
	// Author: monsterfei
	// Date: 2026-07-01
	ErrUserDisabled = errors.New("user is disabled")
)

// UserProfile contains safe user dashboard profile metadata.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-07-01
type UserProfile struct {
	UserID                  int64                             `json:"user_id"`
	TelegramBound           bool                              `json:"telegram_bound"`
	TelegramChatIDMasked    string                            `json:"telegram_chat_id_masked,omitempty"`
	TelegramDeliveryEnabled bool                              `json:"telegram_delivery_enabled"`
	NotificationPreferences model.UserNotificationPreferences `json:"notification_preferences"`
	RecentDeliveryStatus    string                            `json:"recent_delivery_status,omitempty"`
	Plan                    string                            `json:"plan"`
	Status                  string                            `json:"status"`
	Limits                  authsvc.PlanLimits                `json:"limits"`
}

// UserNotificationLog contains safe notification delivery metadata for dashboard users.
//
// Author: monsterfei
// Date: 2026-07-01
type UserNotificationLog struct {
	AlertID      string    `json:"alert_id"`
	Channel      string    `json:"channel"`
	Target       string    `json:"target"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserService defines user dashboard reads required by API handlers.
//
// Author: monsterfei
// Date: 2026-06-30
type UserService interface {
	Profile(context.Context, int64) (UserProfile, error)
	ListAlerts(context.Context, int64, int) ([]model.Alert, error)
	ListNotificationLogs(context.Context, int64, int) ([]UserNotificationLog, error)
	CanCreateRule(context.Context, model.User) error
	AlertHistoryLimit(model.User, int) int
	UpdateTelegramDeliveryEnabled(context.Context, int64, bool) (UserProfile, error)
	UpdateTelegramNotificationPreferences(context.Context, int64, model.UserNotificationPreferences) (UserProfile, error)
	UnbindTelegram(context.Context, int64) (UserProfile, error)
}

// TelegramBindingService defines user-owned Telegram binding-token creation.
//
// Author: monsterfei
// Date: 2026-07-01
type TelegramBindingService interface {
	CreateBindingToken(context.Context, int64) (string, time.Time, error)
}

// TelegramBindingTokenResponse contains a raw short-lived binding token.
//
// Author: monsterfei
// Date: 2026-07-01
type TelegramBindingTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// telegramDeliveryPreferenceRequest contains a Telegram delivery toggle payload.
//
// Author: monsterfei
// Date: 2026-07-01
type telegramDeliveryPreferenceRequest struct {
	Enabled *bool `json:"enabled"`
}

// mountUserRoutes attaches protected user-facing dashboard APIs.
//
// Author: monsterfei
// Date: 2026-06-30
// @param mux HTTP multiplexer to receive routes.
// @param deps Runtime dependencies required by user APIs.
func mountUserRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleUserProfile(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/user/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleUserAlertsList(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/user/notifications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleUserNotificationsList(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/user/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleUserRulesList(deps).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost {
			handleUserRulesPost(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/user/telegram/binding-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handleTelegramBindingToken(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/user/telegram/binding", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handleTelegramUnbind(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/user/telegram/delivery", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleTelegramDeliveryPreferenceGet(deps).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPut {
			handleTelegramDeliveryPreferencePut(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/user/telegram/preferences", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleTelegramNotificationPreferencesGet(deps).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPut {
			handleTelegramNotificationPreferencesPut(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
}

// handleUserProfile returns safe profile and binding state for one user.
//
// Author: monsterfei
// Date: 2026-06-30
// @param deps Runtime dependencies required by the handler.
// @returns HTTP handler for user profile reads.
func handleUserProfile(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"code":    501,
				"message": "user service is not configured",
				"data":    nil,
			})
			return
		}
		profile, err := deps.User.Profile(r.Context(), currentUser.ID)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data":    profile,
		})
	}
}

// requireDashboardUser resolves an active session user or writes the API error response.
//
// Author: monsterfei
// Date: 2026-07-01
// @param w HTTP response writer.
// @param r HTTP request containing session cookie.
// @param deps Runtime dependencies required by user APIs.
// @returns Current user, whether one was authenticated, and lookup error.
func requireDashboardUser(w http.ResponseWriter, r *http.Request, deps Dependencies) (model.User, bool, error) {
	currentUser, ok, err := requireUser(r, deps)
	if err != nil {
		writeInternalError(w, err)
		return model.User{}, false, err
	}
	if !ok {
		writeUnauthorized(w)
		return model.User{}, false, nil
	}
	if currentUser.Status == model.UserStatusDisabled {
		writeJSON(w, http.StatusForbidden, map[string]any{"code": 403, "message": ErrUserDisabled.Error(), "data": nil})
		return model.User{}, false, nil
	}
	return currentUser, true, nil
}

// handleUserAlertsList returns alert history delivered to one user.
//
// Author: monsterfei
// Date: 2026-06-30
// @param deps Runtime dependencies required by the handler.
// @returns HTTP handler for user alert history reads.
func handleUserAlertsList(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"code":    501,
				"message": "user service is not configured",
				"data":    nil,
			})
			return
		}
		limit := deps.User.AlertHistoryLimit(currentUser, userLimitFromQuery(r))
		alerts, err := deps.User.ListAlerts(r.Context(), currentUser.ID, limit)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data":    alerts,
		})
	}
}

// handleUserNotificationsList returns notification delivery logs for one session user.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by the handler.
// @returns HTTP handler for user notification-log reads.
func handleUserNotificationsList(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"code":    501,
				"message": "user service is not configured",
				"data":    nil,
			})
			return
		}
		logs, err := deps.User.ListNotificationLogs(r.Context(), currentUser.ID, userLimitFromQuery(r))
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data":    logs,
		})
	}
}

// handleUserRulesList returns one user's personal rules.
//
// Author: monsterfei
// Date: 2026-06-30
// @param deps Runtime dependencies required by the handler.
// @returns HTTP handler for user rule reads.
// modified by monsterfei on 2026-07-02
func handleUserRulesList(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.Rules == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"code":    501,
				"message": "rule service is not configured",
				"data":    nil,
			})
			return
		}
		rules, err := deps.Rules.ListUserRules(r.Context(), currentUser.ID)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data":    nonNilSlice(rules),
		})
	}
}

// handleUserRulesPost writes one user-owned personal rule.
//
// Author: monsterfei
// Date: 2026-06-30
// @param deps Runtime dependencies required by the handler.
// @returns HTTP handler for user rule writes.
func handleUserRulesPost(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.Rules == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"code":    501,
				"message": "rule service is not configured",
				"data":    nil,
			})
			return
		}

		var req ruleWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":    400,
				"message": "invalid json body",
				"data":    nil,
			})
			return
		}
		req.UserID = &currentUser.ID
		rule, err := req.toModel()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":    400,
				"message": err.Error(),
				"data":    nil,
			})
			return
		}
		if deps.User != nil {
			if err := deps.User.CanCreateRule(r.Context(), currentUser); err != nil {
				if errors.Is(err, ErrSubscriptionRuleLimitExceeded) || errors.Is(err, ErrUserDisabled) {
					writeJSON(w, http.StatusForbidden, map[string]any{"code": 403, "message": err.Error(), "data": nil})
					return
				}
				writeInternalError(w, err)
				return
			}
		}
		if err := deps.Rules.UpsertUserRule(r.Context(), rule); err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "rule updated",
			"data":    rule,
		})
	}
}

// handleTelegramBindingToken creates a short-lived Telegram binding token for the session user.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by user APIs.
// @returns HTTP handler for Telegram binding-token creation.
func handleTelegramBindingToken(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.TelegramBinding == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"code":    501,
				"message": "telegram binding service is not configured",
				"data":    nil,
			})
			return
		}
		token, expiresAt, err := deps.TelegramBinding.CreateBindingToken(r.Context(), currentUser.ID)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "telegram binding token created",
			"data": TelegramBindingTokenResponse{
				Token:     token,
				ExpiresAt: expiresAt,
			},
		})
	}
}

// handleTelegramDeliveryPreferenceGet returns Telegram delivery preference for the session user.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by user APIs.
// @returns HTTP handler for Telegram delivery preference reads.
func handleTelegramDeliveryPreferenceGet(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"code": 501, "message": "user service is not configured", "data": nil})
			return
		}
		profile, err := deps.User.Profile(r.Context(), currentUser.ID)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": profile})
	}
}

// handleTelegramDeliveryPreferencePut updates Telegram delivery preference for the session user.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by user APIs.
// @returns HTTP handler for Telegram delivery preference writes.
func handleTelegramDeliveryPreferencePut(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"code": 501, "message": "user service is not configured", "data": nil})
			return
		}
		var req telegramDeliveryPreferenceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "enabled is required", "data": nil})
			return
		}
		profile, err := deps.User.UpdateTelegramDeliveryEnabled(r.Context(), currentUser.ID, *req.Enabled)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "telegram delivery updated", "data": profile})
	}
}

// handleTelegramNotificationPreferencesGet returns Telegram quiet-hours and digest preferences.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by user APIs.
// @returns HTTP handler for Telegram notification preference reads.
func handleTelegramNotificationPreferencesGet(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"code": 501, "message": "user service is not configured", "data": nil})
			return
		}
		profile, err := deps.User.Profile(r.Context(), currentUser.ID)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": profile})
	}
}

// handleTelegramNotificationPreferencesPut updates Telegram quiet-hours and digest preferences.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by user APIs.
// @returns HTTP handler for Telegram notification preference writes.
func handleTelegramNotificationPreferencesPut(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"code": 501, "message": "user service is not configured", "data": nil})
			return
		}
		var req model.UserNotificationPreferences
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid json body", "data": nil})
			return
		}
		req = normalizeTelegramNotificationPreferences(req)
		if err := validateTelegramNotificationPreferences(req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": err.Error(), "data": nil})
			return
		}
		profile, err := deps.User.UpdateTelegramNotificationPreferences(r.Context(), currentUser.ID, req)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "telegram preferences updated", "data": profile})
	}
}

// handleTelegramUnbind clears Telegram binding for the current session user.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by user APIs.
// @returns HTTP handler for Telegram unbind.
func handleTelegramUnbind(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		if deps.User == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"code": 501, "message": "user service is not configured", "data": nil})
			return
		}
		profile, err := deps.User.UnbindTelegram(r.Context(), currentUser.ID)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "telegram unbound", "data": profile})
	}
}

// requiredUserIDFromQuery parses a required positive user id from query parameters.
//
// Author: monsterfei
// Date: 2026-06-30
// @param r HTTP request containing user_id.
// @returns Parsed user id or validation error.
func requiredUserIDFromQuery(r *http.Request) (int64, error) {
	userID, ok, err := userIDFromQuery(r)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errors.New("user_id is required")
	}
	return userID, nil
}

// userLimitFromQuery parses a bounded user list limit.
//
// Author: monsterfei
// Date: 2026-06-30
// @param r HTTP request containing optional limit.
// @returns Safe bounded list limit.
func userLimitFromQuery(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// normalizeTelegramNotificationPreferences applies safe defaults to optional preference fields.
//
// Author: monsterfei
// Date: 2026-07-01
// @param preferences Telegram quiet-hours and digest preference payload.
// @returns Preference payload with default quiet-hours and digest interval values.
func normalizeTelegramNotificationPreferences(preferences model.UserNotificationPreferences) model.UserNotificationPreferences {
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

// validateTelegramNotificationPreferences checks quiet-hours and digest bounds before persistence.
//
// Author: monsterfei
// Date: 2026-07-01
// @param preferences Telegram quiet-hours and digest preference payload.
// @returns Error when a payload value is invalid.
// modified by monsterfei on 2026-08-29
func validateTelegramNotificationPreferences(preferences model.UserNotificationPreferences) error {
	if len(preferences.TelegramQuietHoursStart) != 5 || preferences.TelegramQuietHoursStart[2] != ':' {
		return errors.New("telegram_quiet_hours_start must use HH:MM")
	}
	if _, err := time.Parse("15:04", preferences.TelegramQuietHoursStart); err != nil {
		return errors.New("telegram_quiet_hours_start must use HH:MM")
	}
	if len(preferences.TelegramQuietHoursEnd) != 5 || preferences.TelegramQuietHoursEnd[2] != ':' {
		return errors.New("telegram_quiet_hours_end must use HH:MM")
	}
	if _, err := time.Parse("15:04", preferences.TelegramQuietHoursEnd); err != nil {
		return errors.New("telegram_quiet_hours_end must use HH:MM")
	}
	if _, err := time.LoadLocation(preferences.TelegramQuietHoursTimezone); err != nil {
		return errors.New("telegram_quiet_hours_timezone is invalid")
	}
	if preferences.TelegramDigestIntervalMin < 5 || preferences.TelegramDigestIntervalMin > 1440 {
		return errors.New("telegram_digest_interval_min must be between 5 and 1440")
	}
	return nil
}
