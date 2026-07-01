package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// TelegramBindingRepo persists expiring Telegram account binding tokens.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type TelegramBindingRepo struct {
	DB *pgxpool.Pool
}

// Create stores one Telegram binding token hash.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param token Telegram binding token to persist.
// @returns Error when persistence fails.
func (r TelegramBindingRepo) Create(ctx context.Context, token model.TelegramBindingToken) error {
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO telegram_binding_tokens (
			user_id,
			token_hash,
			expires_at,
			used_at,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, token.UserID, token.TokenHash, token.ExpiresAt, token.UsedAt, token.CreatedAt)
	return err
}

// FindActiveByHash returns one unexpired unused Telegram binding token by hash.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param tokenHash SHA-256 token hash.
// @param now Current time for expiry comparison.
// @returns Telegram binding token model, whether it was found, and query error.
func (r TelegramBindingRepo) FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.TelegramBindingToken, bool, error) {
	var token model.TelegramBindingToken
	err := r.DB.QueryRow(ctx, `
		SELECT id,
			user_id,
			token_hash,
			expires_at,
			used_at,
			created_at
		FROM telegram_binding_tokens
		WHERE token_hash = $1
			AND used_at IS NULL
			AND expires_at > $2
		LIMIT 1
	`, tokenHash, now).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TelegramBindingToken{}, false, nil
	}
	if err != nil {
		return model.TelegramBindingToken{}, false, err
	}
	return token, true, nil
}

// MarkUsed consumes one Telegram binding token hash.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param tokenHash SHA-256 token hash.
// @param now Usage timestamp.
// @returns Error when persistence fails.
func (r TelegramBindingRepo) MarkUsed(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE telegram_binding_tokens
		SET used_at = $2
		WHERE token_hash = $1
	`, tokenHash, now)
	return err
}
