package rule

import (
	"context"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type stubRuleRepository struct {
	enabledRules []model.AlertRule
	systemRules  []model.AlertRule
	upserted     []model.AlertRule
}

func (s *stubRuleRepository) ListEnabled(context.Context) ([]model.AlertRule, error) {
	return append([]model.AlertRule(nil), s.enabledRules...), nil
}

func (s *stubRuleRepository) ListSystemRules(context.Context) ([]model.AlertRule, error) {
	return append([]model.AlertRule(nil), s.systemRules...), nil
}

func (s *stubRuleRepository) UpsertSystemRule(_ context.Context, rule model.AlertRule) error {
	s.upserted = append(s.upserted, rule)
	return nil
}

func TestRuntimeRuleServiceLoadAppliesDatabaseOverrides(t *testing.T) {
	engine := NewEngine(Config{
		LargeTradeThreshold:  200000,
		LiquidationThreshold: 200000,
		FundingAbsThreshold:  2,
	})
	service := NewRuntimeRuleService(&stubRuleRepository{
		systemRules: []model.AlertRule{
			{
				Exchange:  "binance",
				Symbol:    "BTCUSDT",
				RuleType:  "large_trade",
				Threshold: 100000,
				Enabled:   true,
			},
		},
	}, engine)

	if err := service.Load(context.Background()); err != nil {
		t.Fatalf("load rules: %v", err)
	}

	alerts := engine.Evaluate(model.MarketEvent{
		ID:         "event-1",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "BTCUSDT",
		EventType:  "agg_trade",
		Side:       "Aggressive Buy",
		Price:      100000,
		Quantity:   1.5,
		Notional:   150000,
		EventTime:  time.Now().UTC(),
	})
	if len(alerts) != 1 || alerts[0].Type != "large_trade" {
		t.Fatalf("expected large trade alert from loaded override, got %+v", alerts)
	}
}

func TestRuntimeRuleServiceUpsertUpdatesRuntimeEngine(t *testing.T) {
	engine := NewEngine(Config{
		LargeTradeThreshold:  100000,
		LiquidationThreshold: 100000,
		FundingAbsThreshold:  2,
	})
	repo := &stubRuleRepository{}
	service := NewRuntimeRuleService(repo, engine)

	baseEvent := model.MarketEvent{
		ID:         "event-2",
		Exchange:   "binance",
		MarketType: "futures",
		Symbol:     "BTCUSDT",
		EventType:  "liquidation",
		Side:       "Long Liquidation",
		Price:      100000,
		Quantity:   2,
		Notional:   200000,
		EventTime:  time.Now().UTC(),
	}
	if alerts := engine.Evaluate(baseEvent); len(alerts) != 1 {
		t.Fatalf("expected default liquidation alert, got %+v", alerts)
	}

	err := service.UpsertSystemRule(context.Background(), model.AlertRule{
		Scope:     "system",
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		RuleType:  "liquidation",
		Threshold: 300000,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("upsert rule: %v", err)
	}

	if len(repo.upserted) != 1 {
		t.Fatalf("expected repo upsert to be called, got %d", len(repo.upserted))
	}
	if alerts := engine.Evaluate(baseEvent); len(alerts) != 0 {
		t.Fatalf("expected runtime engine to honor updated threshold, got %+v", alerts)
	}
}
