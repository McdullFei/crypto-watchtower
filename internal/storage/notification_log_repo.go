package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type NotificationLogRepo struct {
	DB *pgxpool.Pool
}

func (r NotificationLogRepo) Insert(ctx context.Context, log model.NotificationLog) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO notification_logs
			(user_id, alert_id, channel, target, status, error_message, created_at)
		VALUES
			($1,$2,$3,$4,$5,$6,$7)
	`, log.UserID, log.AlertID, log.Channel, log.Target, log.Status, log.ErrorMessage, log.CreatedAt)
	return err
}
