package storage

import (
	"context"
	"fmt"
	"time"

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

func (r NotificationLogRepo) List(ctx context.Context, filter ListFilter) ([]model.NotificationLog, error) {
	query := `
		SELECT id, user_id, alert_id, channel, target, status, error_message, created_at
		FROM notification_logs
		WHERE 1=1
	`
	args := make([]any, 0, 2)
	if filter.Status != "" {
		args = append(args, filter.Status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	query += " ORDER BY created_at DESC"
	args = append(args, normalizedLimit(filter.Limit))
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.NotificationLog
	for rows.Next() {
		var item model.NotificationLog
		if err := rows.Scan(&item.ID, &item.UserID, &item.AlertID, &item.Channel, &item.Target, &item.Status, &item.ErrorMessage, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r NotificationLogRepo) CountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM notification_logs WHERE created_at >= $1`, since).Scan(&count)
	return count, err
}
