package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
