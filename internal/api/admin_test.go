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
	rules         []model.AlertRule
	alerts        []model.Alert
	events        []model.MarketEvent
	notifications []model.NotificationLog
}

func (s stubAdminService) Overview(context.Context) (AdminOverview, error) {
	return s.overview, nil
}

func (s stubAdminService) ListRules(context.Context, AdminListFilter) ([]model.AlertRule, error) {
	return s.rules, nil
}

func (s stubAdminService) ListAlerts(context.Context, AdminListFilter) ([]model.Alert, error) {
	return s.alerts, nil
}

func (s stubAdminService) ListEvents(context.Context, AdminListFilter) ([]model.MarketEvent, error) {
	return s.events, nil
}

func (s stubAdminService) ListNotifications(context.Context, AdminListFilter) ([]model.NotificationLog, error) {
	return s.notifications, nil
}

func TestAdminOverviewRequiresBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin:          stubAdminService{},
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
		Admin: stubAdminService{
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

func TestAdminAlertsReturnsList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/alerts?symbol=BTCUSDT&limit=10", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Admin: stubAdminService{
			alerts: []model.Alert{
				{ID: "alert-1", Symbol: "BTCUSDT", Type: "large_trade", Title: "Large trade", CreatedAt: time.Unix(1710000000, 0).UTC()},
			},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "alert-1") {
		t.Fatalf("expected alert id in response: %s", rec.Body.String())
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
