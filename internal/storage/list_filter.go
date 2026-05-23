package storage

type ListFilter struct {
	Symbol    string
	RuleType  string
	EventType string
	Status    string
	Limit     int
}
