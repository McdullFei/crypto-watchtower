package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	listRules []model.AlertRule
	upserted  *model.AlertRule
}

func (s *stubRuleService) ListEnabled(context.Context) ([]model.AlertRule, error) {
	return s.listRules, nil
}

func (s *stubRuleService) UpsertSystemRule(_ context.Context, rule model.AlertRule) error {
	copy := rule
	s.upserted = &copy
	return nil
}

func TestHealthHandlerReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHealthHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
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
