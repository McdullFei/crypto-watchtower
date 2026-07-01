package rule

import (
	"context"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type RuleRepository interface {
	ListEnabled(context.Context) ([]model.AlertRule, error)
	ListSystemRules(context.Context) ([]model.AlertRule, error)
	ListUserRules(context.Context, int64) ([]model.AlertRule, error)
	CountUserRules(context.Context, int64) (int64, error)
	UpsertSystemRule(context.Context, model.AlertRule) error
	UpsertUserRule(context.Context, model.AlertRule) error
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

// ListUserRules returns rules owned by one user.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id to filter.
// @returns User-scoped alert rules.
func (s *RuntimeRuleService) ListUserRules(ctx context.Context, userID int64) ([]model.AlertRule, error) {
	return s.repo.ListUserRules(ctx, userID)
}

// CountUserRules returns how many user-scoped rules belong to one user.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to count.
// @returns Number of user-owned rules.
func (s *RuntimeRuleService) CountUserRules(ctx context.Context, userID int64) (int64, error) {
	return s.repo.CountUserRules(ctx, userID)
}

func (s *RuntimeRuleService) UpsertSystemRule(ctx context.Context, rule model.AlertRule) error {
	if err := s.repo.UpsertSystemRule(ctx, rule); err != nil {
		return err
	}
	s.engine.ApplyRule(rule)
	return nil
}

// UpsertUserRule persists a user rule without mutating the global system rule engine.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param rule User-scoped alert rule.
// @returns Error when persistence fails.
func (s *RuntimeRuleService) UpsertUserRule(ctx context.Context, rule model.AlertRule) error {
	return s.repo.UpsertUserRule(ctx, rule)
}
