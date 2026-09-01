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

// List returns notification logs matching the provided bounded filter.
//
// Author: monsterfei
// Date: 2026-08-31
// @param ctx Request context.
// @param filter Bounded notification log filter.
// @returns Matching notification logs or a persistence error.
func (r NotificationLogRepo) List(ctx context.Context, filter ListFilter) ([]model.NotificationLog, error) {
	query := `
		SELECT id, user_id, alert_id, channel, target, status, COALESCE(error_message, ''), created_at
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

// LatestForUser returns recent notification logs owned by one user.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id to filter.
// @param limit Maximum number of rows to return.
// @returns Bounded recent notification logs.
// modified by monsterfei on 2026-08-31
func (r NotificationLogRepo) LatestForUser(ctx context.Context, userID int64, limit int) ([]model.NotificationLog, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT id, user_id, alert_id, channel, target, status, COALESCE(error_message, ''), created_at
		FROM notification_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, normalizedLimit(limit))
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

// CountByStatusSince returns notification counts grouped by delivery status after the given time.
//
// Author: monsterfei
// Date: 2026-06-29
func (r NotificationLogRepo) CountByStatusSince(ctx context.Context, since time.Time) (map[string]int64, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT status, COUNT(*) AS count
		FROM notification_logs
		WHERE created_at >= $1
		GROUP BY status
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out[status] = count
	}
	return out, rows.Err()
}
