package storage

type ListFilter struct {
	Exchange  string
	Symbol    string
	RuleType  string
	EventType string
	Status    string
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
