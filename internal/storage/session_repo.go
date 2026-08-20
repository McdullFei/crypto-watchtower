package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// SessionRepo persists user login sessions.
//
// Author: monsterfei
// Date: 2026-07-01
type SessionRepo struct {
	DB *pgxpool.Pool
}

// Create stores one login session token hash.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param session Session to persist.
// @returns Error when persistence fails.
func (r SessionRepo) Create(ctx context.Context, session model.UserSession) error {
	now := time.Now().UTC()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO user_sessions (
			user_id,
			token_hash,
			expires_at,
			revoked_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, session.UserID, session.TokenHash, session.ExpiresAt, session.RevokedAt, session.CreatedAt, session.UpdatedAt)
	return err
}

// FindActiveByHash returns one unexpired active session by token hash.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param tokenHash SHA-256 token hash.
// @param now Current time for expiry comparison.
// @returns Session model, whether it was found, and query error.
func (r SessionRepo) FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.UserSession, bool, error) {
	var session model.UserSession
	err := r.DB.QueryRow(ctx, `
		SELECT id,
			user_id,
			token_hash,
			expires_at,
			revoked_at,
			created_at,
			updated_at
		FROM user_sessions
		WHERE token_hash = $1
			AND revoked_at IS NULL
			AND expires_at > $2
		LIMIT 1
	`, tokenHash, now).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.ExpiresAt, &session.RevokedAt, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.UserSession{}, false, nil
	}
	if err != nil {
		return model.UserSession{}, false, err
	}
	return session, true, nil
}

// RevokeByHash marks one session token hash as revoked.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param tokenHash SHA-256 token hash.
// @param now Revocation timestamp.
// @returns Error when persistence fails.
func (r SessionRepo) RevokeByHash(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = $2,
			updated_at = $2
		WHERE token_hash = $1
	`, tokenHash, now)
	return err
}
