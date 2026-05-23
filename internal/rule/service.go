package rule

import (
	"context"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type RuleRepository interface {
	ListEnabled(context.Context) ([]model.AlertRule, error)
	ListSystemRules(context.Context) ([]model.AlertRule, error)
	UpsertSystemRule(context.Context, model.AlertRule) error
}

type RuntimeRuleService struct {
	repo   RuleRepository
	engine *Engine
}

func NewRuntimeRuleService(repo RuleRepository, engine *Engine) *RuntimeRuleService {
	return &RuntimeRuleService{
		repo:   repo,
		engine: engine,
	}
}

func (s *RuntimeRuleService) Load(ctx context.Context) error {
	rules, err := s.repo.ListSystemRules(ctx)
	if err != nil {
		return err
	}
	s.engine.LoadRules(rules)
	return nil
}

func (s *RuntimeRuleService) ListEnabled(ctx context.Context) ([]model.AlertRule, error) {
	return s.repo.ListEnabled(ctx)
}

func (s *RuntimeRuleService) UpsertSystemRule(ctx context.Context, rule model.AlertRule) error {
	if err := s.repo.UpsertSystemRule(ctx, rule); err != nil {
		return err
	}
	s.engine.ApplyRule(rule)
	return nil
}
