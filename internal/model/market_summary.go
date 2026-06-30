package model

import "time"

// MarketSummary stores one generated market summary window.
//
// Author: monsterfei
// Date: 2026-06-30
type MarketSummary struct {
	ID           string
	WindowFrom   time.Time
	WindowTo     time.Time
	Provider     string
	Status       string
	Content      string
	ErrorMessage string
	CreatedAt    time.Time
}
