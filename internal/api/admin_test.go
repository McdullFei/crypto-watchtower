package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type stubAdminService struct {
	overview      AdminOverview
	trends        AdminTrends
	rules         []model.AlertRule
	alerts        []model.Alert
	events        []model.MarketEvent
	notifications []model.NotificationLog
	lastFilter    AdminListFilter
}

func (s stubAdminService) Overview(context.Context) (AdminOverview, error) {
	return s.overview, nil
}

func (s *stubAdminService) Trends(context.Context) (AdminTrends, error) {
	return s.trends, nil
}

func (s *stubAdminService) ListRules(_ context.Context, filter AdminListFilter) ([]model.AlertRule, error) {
	s.lastFilter = filter
	return s.rules, nil
}

func (s *stubAdminService) ListAlerts(_ context.Context, filter AdminListFilter) ([]model.Alert, error) {
	s.lastFilter = filter
	return s.alerts, nil
}

func (s *stubAdminService) ListEvents(_ context.Context, filter AdminListFilter) ([]model.MarketEvent, error) {
	s.lastFilter = filter
	return s.events, nil
}

func (s *stubAdminService) ListNotifications(_ context.Context, filter AdminListFilter) ([]model.NotificationLog, error) {
	s.lastFilter = filter
	return s.notifications, nil
}

func TestAdminOverviewRequiresBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin:          &stubAdminService{},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestAdminOverviewReturnsOverview(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin: &stubAdminService{
			overview: AdminOverview{
				RuleCount:         4,
				AlertCount24h:     7,
				EventCount24h:     22,
				NotificationCount: 9,
				LastAlertAt:       timePtr(time.Unix(1710000000, 0).UTC()),
			},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data AdminOverview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.RuleCount != 4 || payload.Data.AlertCount24h != 7 {
		t.Fatalf("unexpected overview payload: %+v", payload.Data)
	}
}

// TestAdminTrendsReturnsSummary verifies the Admin Trends API exposes lightweight operational counters.
//
// Author: monsterfei
// Date: 2026-06-29
func TestAdminTrendsReturnsSummary(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/trends", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin: &stubAdminService{
			trends: AdminTrends{
				Alerts24h: 11,
				Notifications24h: AdminNotificationTrend{
					Sent:   9,
					Failed: 2,
				},
				SymbolAlerts24h: []AdminSymbolCount{
					{Symbol: "BTCUSDT", Count: 7},
				},
			},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "BTCUSDT") || !strings.Contains(rec.Body.String(), `"failed":2`) {
		t.Fatalf("unexpected trends response: %s", rec.Body.String())
	}
}

func TestAdminAlertsReturnsList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/alerts?symbol=BTCUSDT&limit=10", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	admin := &stubAdminService{
		alerts: []model.Alert{
			{ID: "alert-1", Symbol: "BTCUSDT", Type: "large_trade", Title: "Large trade", CreatedAt: time.Unix(1710000000, 0).UTC()},
		},
	}

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin:          admin,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "alert-1") {
		t.Fatalf("expected alert id in response: %s", rec.Body.String())
	}
	if admin.lastFilter.Symbol != "BTCUSDT" || admin.lastFilter.Limit != 10 {
		t.Fatalf("unexpected admin filter: %+v", admin.lastFilter)
	}
}

// TestAdminAlertsParsesExchangeFilter verifies Admin list APIs accept exchange.
//
// Author: monsterfei
// Date: 2026-06-29
func TestAdminAlertsParsesExchangeFilter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/alerts?exchange=okx&symbol=BTCUSDT&limit=10", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	admin := &stubAdminService{}

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin:          admin,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if admin.lastFilter.Exchange != "okx" || admin.lastFilter.Symbol != "BTCUSDT" {
		t.Fatalf("unexpected admin filter: %+v", admin.lastFilter)
	}
}

// TestAdminRulesParsesUserRuleFilters verifies Admin rules can filter user-scoped rules.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestAdminRulesParsesUserRuleFilters(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/rules?scope=user&user_id=42&limit=10", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	admin := &stubAdminService{}

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin:          admin,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if admin.lastFilter.Scope != "user" || admin.lastFilter.UserID == nil || *admin.lastFilter.UserID != 42 {
		t.Fatalf("unexpected admin filter: %+v", admin.lastFilter)
	}
}

// TestAdminEventsReturnsList verifies the Admin Events API exposes alert-related market events.
//
// Author: monsterfei
// Date: 2026-06-29
func TestAdminEventsReturnsList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events?symbol=BTCUSDT&event_type=agg_trade&limit=10", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	admin := &stubAdminService{
		events: []model.MarketEvent{
			{ID: "event-1", Symbol: "BTCUSDT", EventType: "agg_trade", MarketType: "spot", Notional: 123456, EventTime: time.Unix(1710000000, 0).UTC()},
		},
	}

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin:          admin,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "event-1") {
		t.Fatalf("expected event id in response: %s", rec.Body.String())
	}
	if admin.lastFilter.Symbol != "BTCUSDT" || admin.lastFilter.EventType != "agg_trade" || admin.lastFilter.Limit != 10 {
		t.Fatalf("unexpected admin filter: %+v", admin.lastFilter)
	}
}

func TestAdminPageIsServed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "CryptoWatchtower Admin") {
		t.Fatalf("expected admin page body, got %s", rec.Body.String())
	}
}

// TestAdminPageIncludesAlertEventsPanel verifies the admin console exposes alert-related market events.
//
// Author: monsterfei
// Date: 2026-06-29
func TestAdminPageIncludesAlertEventsPanel(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Alert Events") {
		t.Fatalf("expected alert events panel in admin page, got %s", rec.Body.String())
	}
}

// TestAdminPageIncludesPhase2AControls verifies the admin console exposes the Phase 2-A operator controls.
//
// Author: monsterfei
// Date: 2026-06-29
func TestAdminPageIncludesPhase2AControls(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, expected := range []string{
		"Runtime Status",
		"List Filters",
		"Rule Editor",
		"Trend Summary",
		"filter-symbol",
		"rule-threshold",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in admin page, got %s", expected, body)
		}
	}
}

// TestAdminPageIncludesLanguageControls verifies the admin console exposes bilingual controls.
//
// Author: monsterfei
// Date: 2026-06-29
func TestAdminPageIncludesLanguageControls(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, expected := range []string{
		`id="language-select"`,
		`data-i18n="language.label"`,
		`data-i18n="panel.runtime"`,
		`<option value="zh">中文</option>`,
		`<option value="en">English</option>`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in admin page, got %s", expected, body)
		}
	}
}

// TestAdminScriptIncludesBilingualDictionary verifies dynamic admin text can switch languages.
//
// Author: monsterfei
// Date: 2026-06-29
func TestAdminScriptIncludesBilingualDictionary(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/app.js", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, expected := range []string{
		"cw-admin-language",
		"const translations",
		"applyLanguage",
		"运营后台",
		"Monitoring Console",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in admin script, got %s", expected, body)
		}
	}
}
