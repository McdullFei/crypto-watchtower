package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type RuleService interface {
	ListEnabled(context.Context) ([]model.AlertRule, error)
	UpsertSystemRule(context.Context, model.AlertRule) error
}

type ruleWriteRequest struct {
	Exchange  string  `json:"exchange"`
	Symbol    string  `json:"symbol"`
	RuleType  string  `json:"rule_type"`
	Threshold float64 `json:"threshold"`
	WindowSec int     `json:"window_sec"`
	Enabled   *bool   `json:"enabled"`
}

func handleGetRules(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{
			"default_rules": defaultRuleConfigToRecords(deps.RuleConfig, deps.Symbols),
		}
		if deps.Rules != nil {
			rules, err := deps.Rules.ListEnabled(r.Context())
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"code":    500,
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
		if err := deps.Rules.UpsertSystemRule(r.Context(), rule); err != nil {
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
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	if r.WindowSec == 0 {
		r.WindowSec = 60
	}
	now := time.Now().UTC()
	return model.AlertRule{
		Scope:     "system",
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

func defaultRuleConfigToRecords(cfg config.RulesConfig, symbols []string) []model.AlertRule {
	out := make([]model.AlertRule, 0, len(symbols)*3)
	for _, symbol := range symbols {
		out = append(out,
			model.AlertRule{Scope: "system", Exchange: "binance", Symbol: symbol, RuleType: "large_trade", Threshold: cfg.LargeTradeSingleUSDT, WindowSec: 60, Enabled: true},
			model.AlertRule{Scope: "system", Exchange: "binance", Symbol: symbol, RuleType: "liquidation", Threshold: cfg.LiquidationUSDT, WindowSec: 60, Enabled: true},
			model.AlertRule{Scope: "system", Exchange: "binance", Symbol: symbol, RuleType: "funding_anomaly", Threshold: cfg.FundingAbsPercent, WindowSec: 60, Enabled: true},
		)
	}
	return out
}
