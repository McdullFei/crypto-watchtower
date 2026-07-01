package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type stubSender struct {
	alerts []string
}

func (s *stubSender) Send(_ context.Context, alert model.Alert) error {
	s.alerts = append(s.alerts, alert.Title)
	return nil
}

type stubRuleService struct {
	listRules  []model.AlertRule
	userRules  []model.AlertRule
	userCount  int64
	upserted   *model.AlertRule
	userRule   *model.AlertRule
	lastUserID int64
}

type stubHealthCollector struct{}

func (stubHealthCollector) Status() CollectorStatus {
	return CollectorStatus{
		Name:          "binance-spot",
		Connected:     true,
		Reconnects:    1,
		LastEventAt:   timePtr(time.Unix(1710000000, 0).UTC()),
		LastError:     "",
		Subscribed:    []string{"BTCUSDT"},
		LastConnectAt: timePtr(time.Unix(1710000001, 0).UTC()),
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func (s *stubRuleService) ListEnabled(context.Context) ([]model.AlertRule, error) {
	return s.listRules, nil
}

// ListUserRules returns user-scoped rules for API tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (s *stubRuleService) ListUserRules(_ context.Context, userID int64) ([]model.AlertRule, error) {
	s.lastUserID = userID
	return s.userRules, nil
}

// CountUserRules returns the configured user rule count for API tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubRuleService) CountUserRules(context.Context, int64) (int64, error) {
	return s.userCount, nil
}

func (s *stubRuleService) UpsertSystemRule(_ context.Context, rule model.AlertRule) error {
	copy := rule
	s.upserted = &copy
	return nil
}

// UpsertUserRule records a user-scoped rule for API tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (s *stubRuleService) UpsertUserRule(_ context.Context, rule model.AlertRule) error {
	copy := rule
	s.userRule = &copy
	return nil
}

func TestHealthHandlerReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler(nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestHealthHandlerIncludesCollectorStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler([]CollectorStatusProvider{stubHealthCollector{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload struct {
		Data struct {
			Collectors []CollectorStatus `json:"collectors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if len(payload.Data.Collectors) != 1 || !payload.Data.Collectors[0].Connected {
		t.Fatalf("unexpected collectors payload: %+v", payload.Data.Collectors)
	}
}

func TestHealthHandlerIncludesDependencyStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler(nil, []DependencyStatusProvider{
		stubDependency{name: "postgres"},
		stubDependency{name: "redis"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload struct {
		Data struct {
			Dependencies map[string]DependencyStatus `json:"dependencies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload.Data.Dependencies["postgres"].Status != "ok" {
		t.Fatalf("expected postgres ok, got %+v", payload.Data.Dependencies["postgres"])
	}
	if payload.Data.Dependencies["redis"].Status != "ok" {
		t.Fatalf("expected redis ok, got %+v", payload.Data.Dependencies["redis"])
	}
}

func TestHealthHandlerReportsDependencyError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler(nil, []DependencyStatusProvider{
		stubDependency{name: "postgres", err: errors.New("connection refused")},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload struct {
		Data struct {
			Dependencies map[string]DependencyStatus `json:"dependencies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload.Data.Dependencies["postgres"].Status != "error" {
		t.Fatalf("expected postgres error, got %+v", payload.Data.Dependencies["postgres"])
	}
}

type stubDependency struct {
	name string
	err  error
}

func (s stubDependency) Name() string {
	return s.name
}

func (s stubDependency) Check(context.Context) error {
	return s.err
}

func TestWriteRouteRequiresBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/test", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{APIBearerToken: "secret"}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestTelegramTestRouteInvokesSender(t *testing.T) {
	sender := &stubSender{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Telegram:       sender,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(sender.alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(sender.alerts))
	}
}

func TestAlertsTestRouteInvokesSender(t *testing.T) {
	sender := &stubSender{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Telegram:       sender,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(sender.alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(sender.alerts))
	}
}

func TestRulesGetReturnsDatabaseRules(t *testing.T) {
	ruleSvc := &stubRuleService{
		listRules: []model.AlertRule{{Exchange: "binance", Symbol: "BTCUSDT", RuleType: "large_trade", Threshold: 100000}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		RuleConfig: config.RulesConfig{LargeTradeSingleUSDT: 100000},
		Rules:      ruleSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "database_rules") {
		t.Fatalf("expected database rules in body: %s", body)
	}
}

func TestRulesPostUpsertsRule(t *testing.T) {
	ruleSvc := &stubRuleService{}
	body := []byte(`{"exchange":"binance","symbol":"BTCUSDT","rule_type":"large_trade","threshold":120000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Rules:          ruleSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if ruleSvc.upserted == nil || ruleSvc.upserted.Symbol != "BTCUSDT" {
		t.Fatal("expected rule to be upserted")
	}
}

// TestRulesPostUpsertsUserRule verifies protected rule writes can target one user.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestRulesPostUpsertsUserRule(t *testing.T) {
	ruleSvc := &stubRuleService{}
	body := []byte(`{"user_id":42,"exchange":"binance","symbol":"BTCUSDT","rule_type":"large_trade","threshold":120000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		APIBearerToken: "secret",
		Rules:          ruleSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if ruleSvc.userRule == nil || ruleSvc.userRule.UserID == nil || *ruleSvc.userRule.UserID != 42 || ruleSvc.userRule.Scope != "user" {
		t.Fatalf("expected user rule to be upserted, got %+v", ruleSvc.userRule)
	}
}

// TestRulesGetFiltersUserRules verifies rule reads can request one user's rules.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestRulesGetFiltersUserRules(t *testing.T) {
	userID := int64(42)
	ruleSvc := &stubRuleService{
		userRules: []model.AlertRule{{UserID: &userID, Scope: "user", Exchange: "binance", Symbol: "BTCUSDT", RuleType: "large_trade", Threshold: 100000}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules?user_id=42", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		RuleConfig: config.RulesConfig{LargeTradeSingleUSDT: 100000},
		Rules:      ruleSvc,
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if ruleSvc.lastUserID != 42 {
		t.Fatalf("expected user rule filter, got %d", ruleSvc.lastUserID)
	}
	if body := rec.Body.String(); !strings.Contains(body, "database_rules") || !strings.Contains(body, "BTCUSDT") {
		t.Fatalf("expected user database rules in body: %s", body)
	}
}
