package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type AlertRuleRepo struct {
	DB *pgxpool.Pool
}

func (r AlertRuleRepo) ListEnabled(ctx context.Context) ([]model.AlertRule, error) {
	return r.queryRules(ctx, `
		SELECT id, user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at
		FROM alert_rules WHERE enabled = TRUE ORDER BY id ASC
	`)
}

func (r AlertRuleRepo) ListSystemRules(ctx context.Context) ([]model.AlertRule, error) {
	return r.queryRules(ctx, `
		SELECT id, user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at
		FROM alert_rules WHERE scope = 'system' ORDER BY id ASC
	`)
}

func (r AlertRuleRepo) List(ctx context.Context, filter ListFilter) ([]model.AlertRule, error) {
	query := `
		SELECT id, user_id, scope, exchange, symbol, rule_type, threshold, window_sec, enabled, created_at, updated_at
		FROM alert_rules
		WHERE 1=1
	`
	args := make([]any, 0, 3)
	if filter.Symbol != "" {
		args = append(args, filter.Symbol)
		query += fmt.Sprintf(" AND symbol = $%d", len(args))
	}
	if filter.RuleType != "" {
		args = append(args, filter.RuleType)
		query += fmt.Sprintf(" AND rule_type = $%d", len(args))
	}
	query += " ORDER BY updated_at DESC"
	limit := normalizedLimit(filter.Limit)
	args = append(args, limit)
	query += fmt.Sprintf(" LIMIT $%d", len(args))
	return r.queryRules(ctx, query, args...)
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
