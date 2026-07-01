package model

import "time"

const (
	UserPlanFree = "free"
	UserPlanPro  = "pro"
	UserPlanVIP  = "vip"

	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

// User stores account, notification, and subscription metadata.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// modified by __AUTHOR__ on 2026-06-30
// modified by __AUTHOR__ on 2026-07-01
type User struct {
	ID                      int64
	Email                   string
	PasswordHash            string
	EmailVerified           bool
	TelegramChatID          string
	TelegramDeliveryEnabled bool
	Plan                    string
	Status                  string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// UserSession stores one persisted login session.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type UserSession struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PasswordResetToken stores one expiring password reset token.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type PasswordResetToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// TelegramBindingToken stores one expiring Telegram account binding token.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type TelegramBindingToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
