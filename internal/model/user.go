package model

import "time"

type User struct {
	ID             int64
	Email          string
	TelegramChatID string
	Plan           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
