package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// UserRepo persists account and notification user records.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// modified by __AUTHOR__ on 2026-07-01
type UserRepo struct {
	DB *pgxpool.Pool
}

func (r UserRepo) UpsertTelegramChat(ctx context.Context, chatID string) error {
	now := time.Now().UTC()
	_, err := r.DB.Exec(ctx, `
		INSERT INTO users (telegram_chat_id, created_at, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_chat_id)
		DO UPDATE SET updated_at = EXCLUDED.updated_at
	`, chatID, now, now)
	return err
}

// BindTelegramChat stores one Telegram chat id on an existing user account.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to bind.
// @param chatID Telegram chat id.
// @returns Error when persistence fails.
func (r UserRepo) BindTelegramChat(ctx context.Context, userID int64, chatID string) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE users
		SET telegram_chat_id = $2,
			updated_at = $3
		WHERE id = $1
	`, userID, chatID, time.Now().UTC())
	return err
}

// FindByID returns one user by primary key without loading unrelated rows.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id to look up.
// @returns User model, whether it was found, and query error.
func (r UserRepo) FindByID(ctx context.Context, userID int64) (model.User, bool, error) {
	var user model.User
	err := r.DB.QueryRow(ctx, `
		SELECT id,
			COALESCE(email, ''),
			COALESCE(password_hash, ''),
			email_verified,
			COALESCE(telegram_chat_id, ''),
			telegram_delivery_enabled,
			COALESCE(plan, ''),
			COALESCE(status, ''),
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerified, &user.TelegramChatID, &user.TelegramDeliveryEnabled, &user.Plan, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, err
	}
	return user, true, nil
}

// CreateWithPassword inserts one account user with a precomputed password hash.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param user Account user to create.
// @returns Created user with database defaults applied.
func (r UserRepo) CreateWithPassword(ctx context.Context, user model.User) (model.User, error) {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}
	err := r.DB.QueryRow(ctx, `
		INSERT INTO users (
			email,
			password_hash,
			email_verified,
			telegram_chat_id,
			telegram_delivery_enabled,
			plan,
			status,
			created_at,
			updated_at
		)
		VALUES (
			LOWER($1),
			$2,
			$3,
			NULLIF($4, ''),
			$5,
			COALESCE(NULLIF($6, ''), 'free'),
			COALESCE(NULLIF($7, ''), 'active'),
			$8,
			$9
		)
		RETURNING id,
			COALESCE(email, ''),
			COALESCE(password_hash, ''),
			email_verified,
			COALESCE(telegram_chat_id, ''),
			telegram_delivery_enabled,
			COALESCE(plan, ''),
			COALESCE(status, ''),
			created_at,
			updated_at
	`, user.Email, user.PasswordHash, user.EmailVerified, user.TelegramChatID, telegramDeliveryEnabledForInsert(), user.Plan, user.Status, user.CreatedAt, user.UpdatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerified, &user.TelegramChatID, &user.TelegramDeliveryEnabled, &user.Plan, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return model.User{}, err
	}
	return user, nil
}

// FindByEmail returns one user by case-insensitive email.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param email Email address to look up.
// @returns User model, whether it was found, and query error.
func (r UserRepo) FindByEmail(ctx context.Context, email string) (model.User, bool, error) {
	var user model.User
	err := r.DB.QueryRow(ctx, `
		SELECT id,
			COALESCE(email, ''),
			COALESCE(password_hash, ''),
			email_verified,
			COALESCE(telegram_chat_id, ''),
			telegram_delivery_enabled,
			COALESCE(plan, ''),
			COALESCE(status, ''),
			created_at,
			updated_at
		FROM users
		WHERE email IS NOT NULL
			AND LOWER(email) = LOWER($1)
		LIMIT 1
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerified, &user.TelegramChatID, &user.TelegramDeliveryEnabled, &user.Plan, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, err
	}
	return user, true, nil
}

// UpdateTelegramDeliveryEnabled toggles Telegram delivery for one user account.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to update.
// @param enabled Whether Telegram user-rule delivery is enabled.
// @returns Error when persistence fails.
func (r UserRepo) UpdateTelegramDeliveryEnabled(ctx context.Context, userID int64, enabled bool) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE users
		SET telegram_delivery_enabled = $2,
			updated_at = $3
		WHERE id = $1
	`, userID, enabled, time.Now().UTC())
	return err
}

// telegramDeliveryEnabledForInsert keeps zero-value account creation defaulting to enabled.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @returns Telegram delivery preference for user insertion.
func telegramDeliveryEnabledForInsert() bool {
	return true
}

// UpdatePassword replaces one user's password hash.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to update.
// @param passwordHash New bcrypt password hash.
// @returns Error when the update fails.
func (r UserRepo) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
			updated_at = $3
		WHERE id = $1
	`, userID, passwordHash, time.Now().UTC())
	return err
}
