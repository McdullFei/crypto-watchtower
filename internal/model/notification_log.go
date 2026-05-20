package model

import "time"

type NotificationLog struct {
	ID           int64
	UserID       *int64
	AlertID      string
	Channel      string
	Target       string
	Status       string
	ErrorMessage string
	CreatedAt    time.Time
}
