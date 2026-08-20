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
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-06-30
// modified by monsterfei on 2026-07-01
type User struct {
	ID                         int64
	Email                      string
	PasswordHash               string
	EmailVerified              bool
	TelegramChatID             string
	TelegramDeliveryEnabled    bool
	TelegramQuietHoursEnabled  bool
	TelegramQuietHoursStart    string
	TelegramQuietHoursEnd      string
	TelegramQuietHoursTimezone string
	TelegramDigestEnabled      bool
	TelegramDigestIntervalMin  int
	Plan                       string
	Status                     string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// UserNotificationPreferences stores user-controlled Telegram noise preferences.
//
// Author: monsterfei
// Date: 2026-07-01
type UserNotificationPreferences struct {
	TelegramQuietHoursEnabled  bool   `json:"telegram_quiet_hours_enabled"`
	TelegramQuietHoursStart    string `json:"telegram_quiet_hours_start"`
	TelegramQuietHoursEnd      string `json:"telegram_quiet_hours_end"`
	TelegramQuietHoursTimezone string `json:"telegram_quiet_hours_timezone"`
	TelegramDigestEnabled      bool   `json:"telegram_digest_enabled"`
	TelegramDigestIntervalMin  int    `json:"telegram_digest_interval_min"`
}

// UserSession stores one persisted login session.
//
// Author: monsterfei
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
// Author: monsterfei
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
// Author: monsterfei
// Date: 2026-07-01
type TelegramBindingToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
