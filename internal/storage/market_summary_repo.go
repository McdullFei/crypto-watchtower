package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// MarketSummaryRepo stores and reads generated market summaries.
//
// Author: monsterfei
// Date: 2026-06-30
type MarketSummaryRepo struct {
	DB *pgxpool.Pool
}

// Insert stores one generated market summary.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param summary Market summary to persist.
// @returns Error when persistence fails.
func (r MarketSummaryRepo) Insert(ctx context.Context, summary model.MarketSummary) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO market_summaries
			(id, window_from, window_to, provider, status, content, error_message, created_at)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8)
	`, summary.ID, summary.WindowFrom, summary.WindowTo, summary.Provider, summary.Status, summary.Content, summary.ErrorMessage, summary.CreatedAt)
	return err
}

// List returns recent generated market summaries.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param filter Bounded list filter.
// @returns Market summaries ordered by window end time descending.
func (r MarketSummaryRepo) List(ctx context.Context, filter ListFilter) ([]model.MarketSummary, error) {
	query := `
		SELECT id, window_from, window_to, provider, status, content, error_message, created_at
		FROM market_summaries
		WHERE 1=1
	`
	args := make([]any, 0, 2)
	if !filter.Since.IsZero() {
		args = append(args, filter.Since)
		query += fmt.Sprintf(" AND window_to >= $%d", len(args))
	}
	query += " ORDER BY window_to DESC"
	args = append(args, normalizedLimit(filter.Limit))
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.MarketSummary
	for rows.Next() {
		var item model.MarketSummary
		var errorMessage sql.NullString
		if err := rows.Scan(&item.ID, &item.WindowFrom, &item.WindowTo, &item.Provider, &item.Status, &item.Content, &errorMessage, &item.CreatedAt); err != nil {
			return nil, err
		}
		if errorMessage.Valid {
			item.ErrorMessage = errorMessage.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
