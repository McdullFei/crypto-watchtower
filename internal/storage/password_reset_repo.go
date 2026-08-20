package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// PasswordResetRepo persists expiring password reset tokens.
//
// Author: monsterfei
// Date: 2026-07-01
type PasswordResetRepo struct {
	DB *pgxpool.Pool
}

// Create stores one password reset token hash.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param token Password reset token to persist.
// @returns Error when persistence fails.
func (r PasswordResetRepo) Create(ctx context.Context, token model.PasswordResetToken) error {
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO password_reset_tokens (
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

// FindActiveByHash returns one unexpired unused password reset token by hash.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param tokenHash SHA-256 token hash.
// @param now Current time for expiry comparison.
// @returns Password reset token model, whether it was found, and query error.
func (r PasswordResetRepo) FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.PasswordResetToken, bool, error) {
	var token model.PasswordResetToken
	err := r.DB.QueryRow(ctx, `
		SELECT id,
			user_id,
			token_hash,
			expires_at,
			used_at,
			created_at
		FROM password_reset_tokens
		WHERE token_hash = $1
			AND used_at IS NULL
			AND expires_at > $2
		LIMIT 1
	`, tokenHash, now).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PasswordResetToken{}, false, nil
	}
	if err != nil {
		return model.PasswordResetToken{}, false, err
	}
	return token, true, nil
}

// MarkUsed consumes one password reset token hash.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param tokenHash SHA-256 token hash.
// @param now Usage timestamp.
// @returns Error when persistence fails.
func (r PasswordResetRepo) MarkUsed(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = $2
		WHERE token_hash = $1
	`, tokenHash, now)
	return err
}
