package storage

import "time"

// ListFilter carries optional repository filters for bounded list queries.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-06-30
type ListFilter struct {
	Exchange  string
	Symbol    string
	RuleType  string
	EventType string
	Status    string
	Scope     string
	UserID    *int64
	Since     time.Time
	Limit     int
}

// SymbolCount stores a bounded aggregate count for one market symbol.
//
// Author: monsterfei
// Date: 2026-06-29
type SymbolCount struct {
	Symbol string
	Count  int64
}
