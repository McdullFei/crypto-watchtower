package model

import "time"

type AlertRule struct {
	ID        int64
	UserID    *int64
	Scope     string
	Exchange  string
	Symbol    string
	RuleType  string
	Threshold float64
	WindowSec int
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserRuleTarget combines one user-scoped rule with its owning account.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type UserRuleTarget struct {
	User User
	Rule AlertRule
}
