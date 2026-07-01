package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type AlertRuleRepo struct {
	DB *pgxpool.Pool
}

// ListEnabled returns enabled system rules for global rule responses.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// modified by __AUTHOR__ on 2026-06-30
func (r AlertRuleRepo) ListEnabled(ctx context.Context) ([]model.AlertRule, error) {
	return r.queryRules(ctx, `
		SELECT id, user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at
		FROM alert_rules WHERE enabled = TRUE AND scope = 'system' ORDER BY id ASC
	`)
}

func (r AlertRuleRepo) ListSystemRules(ctx context.Context) ([]model.AlertRule, error) {
	return r.queryRules(ctx, `
		SELECT id, user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at
		FROM alert_rules WHERE scope = 'system' ORDER BY id ASC
	`)
}

// List returns alert rules matching the provided bounded filter.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// modified by __AUTHOR__ on 2026-06-30
func (r AlertRuleRepo) List(ctx context.Context, filter ListFilter) ([]model.AlertRule, error) {
	query := `
		SELECT id, user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at
		FROM alert_rules
		WHERE 1=1
	`
	args := make([]any, 0, 4)
	if filter.Exchange != "" {
		args = append(args, filter.Exchange)
		query += fmt.Sprintf(" AND exchange = $%d", len(args))
	}
	if filter.Symbol != "" {
		args = append(args, filter.Symbol)
		query += fmt.Sprintf(" AND symbol = $%d", len(args))
	}
	if filter.RuleType != "" {
		args = append(args, filter.RuleType)
		query += fmt.Sprintf(" AND rule_type = $%d", len(args))
	}
	if filter.Scope != "" {
		args = append(args, filter.Scope)
		query += fmt.Sprintf(" AND scope = $%d", len(args))
	}
	if filter.UserID != nil {
		args = append(args, *filter.UserID)
		query += fmt.Sprintf(" AND user_id = $%d", len(args))
	}
	query += " ORDER BY updated_at DESC"
	limit := normalizedLimit(filter.Limit)
	args = append(args, limit)
	query += fmt.Sprintf(" LIMIT $%d", len(args))
	return r.queryRules(ctx, query, args...)
}

// ListUserRules returns rules owned by one user.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id to filter.
// @returns User-scoped alert rules.
func (r AlertRuleRepo) ListUserRules(ctx context.Context, userID int64) ([]model.AlertRule, error) {
	return r.List(ctx, ListFilter{Scope: "user", UserID: &userID})
}

// CountUserRules returns how many user-scoped rules belong to one user.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to count.
// @returns Number of user-owned rules.
func (r AlertRuleRepo) CountUserRules(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.DB.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM alert_rules
		WHERE scope = 'user'
			AND user_id = $1
	`, userID).Scan(&count)
	return count, err
}

// ListActiveUserRulesForEvent returns bounded user rules and active bound owners for one event.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// modified by __AUTHOR__ on 2026-07-01
// @param ctx Request context.
// @param event Market event used for exchange and symbol lookup.
// @param limit Maximum number of user rules to scan.
// @returns User-rule targets for fanout.
func (r AlertRuleRepo) ListActiveUserRulesForEvent(ctx context.Context, event model.MarketEvent, limit int) ([]model.UserRuleTarget, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT r.id, r.user_id, r.scope, r.exchange, r.symbol, r.rule_type,
			r.threshold, r.window_sec, r.enabled, r.created_at, r.updated_at,
			u.id, COALESCE(u.email, ''), COALESCE(u.password_hash, ''), u.email_verified,
			COALESCE(u.telegram_chat_id, ''), u.telegram_delivery_enabled,
			COALESCE(u.plan, ''), COALESCE(u.status, ''), u.created_at, u.updated_at
		FROM alert_rules r
		INNER JOIN users u ON u.id = r.user_id
		WHERE r.scope = 'user'
			AND r.enabled = TRUE
			AND r.exchange = $1
			AND r.symbol = $2
			AND u.status = 'active'
			AND COALESCE(u.telegram_chat_id, '') <> ''
		ORDER BY r.updated_at DESC
		LIMIT $3
	`, event.Exchange, event.Symbol, normalizedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.UserRuleTarget{}
	for rows.Next() {
		var target model.UserRuleTarget
		if err := rows.Scan(
			&target.Rule.ID, &target.Rule.UserID, &target.Rule.Scope, &target.Rule.Exchange, &target.Rule.Symbol, &target.Rule.RuleType,
			&target.Rule.Threshold, &target.Rule.WindowSec, &target.Rule.Enabled, &target.Rule.CreatedAt, &target.Rule.UpdatedAt,
			&target.User.ID, &target.User.Email, &target.User.PasswordHash, &target.User.EmailVerified,
			&target.User.TelegramChatID, &target.User.TelegramDeliveryEnabled, &target.User.Plan, &target.User.Status,
			&target.User.CreatedAt, &target.User.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, target)
	}
	return out, rows.Err()
}

func (r AlertRuleRepo) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM alert_rules WHERE enabled = TRUE`).Scan(&count)
	return count, err
}

func (r AlertRuleRepo) queryRules(ctx context.Context, query string, args ...any) ([]model.AlertRule, error) {
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AlertRule
	for rows.Next() {
		var item model.AlertRule
		if err := rows.Scan(&item.ID, &item.UserID, &item.Scope, &item.Exchange, &item.Symbol, &item.RuleType, &item.Threshold, &item.WindowSec, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r AlertRuleRepo) UpsertSystemRule(ctx context.Context, rule model.AlertRule) error {
	tag, err := r.DB.Exec(ctx, `
		UPDATE alert_rules
		SET threshold = $1,
			window_sec = $2,
			enabled = $3,
			updated_at = $4
		WHERE user_id IS NULL
		  AND scope = $5
		  AND exchange = $6
		  AND symbol = $7
		  AND rule_type = $8
	`, rule.Threshold, rule.WindowSec, rule.Enabled, rule.UpdatedAt, rule.Scope, rule.Exchange, rule.Symbol, rule.RuleType)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	_, err = r.DB.Exec(ctx, `
		INSERT INTO alert_rules
			(user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at)
		VALUES
			(NULL, $1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, rule.Scope, rule.Exchange, rule.Symbol, rule.RuleType, rule.Threshold, rule.WindowSec, rule.Enabled, rule.CreatedAt, rule.UpdatedAt)
	return err
}

// UpsertUserRule inserts or updates one user-scoped alert rule.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param rule User-scoped alert rule.
// @returns Error when persistence fails.
func (r AlertRuleRepo) UpsertUserRule(ctx context.Context, rule model.AlertRule) error {
	if rule.UserID == nil || *rule.UserID <= 0 {
		return errors.New("user_id is required for user rule")
	}
	if rule.Scope == "" {
		rule.Scope = "user"
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE alert_rules
		SET threshold = $1,
			window_sec = $2,
			enabled = $3,
			updated_at = $4
		WHERE user_id = $5
		  AND scope = $6
		  AND exchange = $7
		  AND symbol = $8
		  AND rule_type = $9
	`, rule.Threshold, rule.WindowSec, rule.Enabled, rule.UpdatedAt, *rule.UserID, rule.Scope, rule.Exchange, rule.Symbol, rule.RuleType)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	_, err = r.DB.Exec(ctx, `
		INSERT INTO alert_rules
			(user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, *rule.UserID, rule.Scope, rule.Exchange, rule.Symbol, rule.RuleType, rule.Threshold, rule.WindowSec, rule.Enabled, rule.CreatedAt, rule.UpdatedAt)
	return err
}
