package model

import "time"

type Alert struct {
	ID          string
	Exchange    string
	MarketType  string
	Symbol      string
	Type        string
	Severity    string
	Title       string
	Message     string
	EventID     string
	RuleID      string
	TriggerKey  string
	TriggerTime time.Time
	CreatedAt   time.Time
}

func (a Alert) TriggerBucket() string {
	return a.Exchange + ":" + a.Symbol + ":" + a.Type
}
