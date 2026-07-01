package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type RuleService interface {
	ListEnabled(context.Context) ([]model.AlertRule, error)
	ListUserRules(context.Context, int64) ([]model.AlertRule, error)
	UpsertSystemRule(context.Context, model.AlertRule) error
	UpsertUserRule(context.Context, model.AlertRule) error
}

// ruleWriteRequest contains a system or user-scoped rule write payload.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type ruleWriteRequest struct {
	Exchange  string  `json:"exchange"`
	Symbol    string  `json:"symbol"`
	RuleType  string  `json:"rule_type"`
	Threshold float64 `json:"threshold"`
	WindowSec int     `json:"window_sec"`
	Enabled   *bool   `json:"enabled"`
	UserID    *int64  `json:"user_id"`
}

// handleGetRules returns default rules plus persisted system or user rules.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// modified by __AUTHOR__ on 2026-06-30
func handleGetRules(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			"default_rules": defaultRuleConfigToRecords(deps.RuleConfig, deps.Symbols),
		}
		if deps.Rules != nil {
			rules, err := listRulesForRequest(r.Context(), deps.Rules, r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"code":    400,
					"message": err.Error(),
					"data":    nil,
				})
				return
			}
			data["database_rules"] = rules
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data":    data,
		})
	}
}

// handlePostRules writes one protected system or user-scoped alert rule.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// modified by __AUTHOR__ on 2026-06-30
func handlePostRules(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorize(r, deps.APIBearerToken) {
			writeUnauthorized(w)
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
		rule, err := req.toModel()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":    400,
				"message": err.Error(),
				"data":    nil,
			})
			return
		}
		if err := upsertRuleForRequest(r.Context(), deps.Rules, rule); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"code":    500,
				"message": err.Error(),
				"data":    nil,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "rule updated",
			"data":    rule,
		})
	}
}

// toModel converts a rule write request into a persisted alert rule.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @returns Alert rule model or validation error.
// modified by __AUTHOR__ on 2026-06-30
func (r ruleWriteRequest) toModel() (model.AlertRule, error) {
	if r.Exchange == "" {
		r.Exchange = "binance"
	}
	if r.Symbol == "" || r.RuleType == "" {
		return model.AlertRule{}, errors.New("symbol and rule_type are required")
	}
	if r.Threshold <= 0 {
		return model.AlertRule{}, errors.New("threshold must be greater than 0")
	}
	scope := "system"
	if r.UserID != nil {
		if *r.UserID <= 0 {
			return model.AlertRule{}, errors.New("user_id must be greater than 0")
		}
		scope = "user"
	}
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	if r.WindowSec == 0 {
		r.WindowSec = 60
	}
	now := time.Now().UTC()
	return model.AlertRule{
		UserID:    r.UserID,
		Scope:     scope,
		Exchange:  r.Exchange,
		Symbol:    r.Symbol,
		RuleType:  r.RuleType,
		Threshold: r.Threshold,
		WindowSec: r.WindowSec,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// listRulesForRequest returns system or user-scoped rules for the public rules API.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param service Rule service dependency.
// @param r HTTP request containing optional user_id.
// @returns Matching alert rules.
func listRulesForRequest(ctx context.Context, service RuleService, r *http.Request) ([]model.AlertRule, error) {
	userID, ok, err := userIDFromQuery(r)
	if err != nil {
		return nil, err
	}
	if ok {
		return service.ListUserRules(ctx, userID)
	}
	return service.ListEnabled(ctx)
}

// upsertRuleForRequest writes a system or user-scoped rule through the rule service.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param service Rule service dependency.
// @param rule Rule model to persist.
// @returns Error when persistence fails.
func upsertRuleForRequest(ctx context.Context, service RuleService, rule model.AlertRule) error {
	if rule.UserID != nil {
		return service.UpsertUserRule(ctx, rule)
	}
	return service.UpsertSystemRule(ctx, rule)
}

// userIDFromQuery parses an optional positive user id from a rules request.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param r HTTP request.
// @returns Parsed user id, whether it was present, and parse error.
func userIDFromQuery(r *http.Request) (int64, bool, error) {
	raw := r.URL.Query().Get("user_id")
	if raw == "" {
		return 0, false, nil
	}
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || userID <= 0 {
		return 0, false, errors.New("user_id must be greater than 0")
	}
	return userID, true, nil
}

func defaultRuleConfigToRecords(cfg config.RulesConfig, symbols []string) []model.AlertRule {
	out := make([]model.AlertRule, 0, len(symbols)*3)
	for _, symbol := range symbols {
		out = append(out,
			model.AlertRule{Scope: "system", Exchange: "binance", Symbol: symbol, RuleType: "large_trade", Threshold: cfg.LargeTradeSingleUSDT, WindowSec: 60, Enabled: true},
			model.AlertRule{Scope: "system", Exchange: "binance", Symbol: symbol, RuleType: "large_trade_window", Threshold: cfg.LargeTradeWindowUSDT, WindowSec: 60, Enabled: true},
			model.AlertRule{Scope: "system", Exchange: "binance", Symbol: symbol, RuleType: "liquidation", Threshold: cfg.LiquidationUSDT, WindowSec: 60, Enabled: true},
			model.AlertRule{Scope: "system", Exchange: "binance", Symbol: symbol, RuleType: "funding_anomaly", Threshold: cfg.FundingAbsPercent, WindowSec: 60, Enabled: true},
		)
	}
	return out
}
