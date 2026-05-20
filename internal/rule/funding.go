package rule

import (
	"fmt"
	"math"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type FundingRule struct {
	AbsThreshold float64
}

func (r FundingRule) Evaluate(event model.MarketEvent) (model.Alert, bool) {
	if event.EventType != "funding" {
		return model.Alert{}, false
	}
	raw, ok := event.Metadata["funding_rate"]
	if !ok {
		return model.Alert{}, false
	}
	rate, ok := raw.(float64)
	if !ok || math.Abs(rate) < r.AbsThreshold {
		return model.Alert{}, false
	}
	return model.Alert{
		ID:          event.ID + "-funding",
		Exchange:    event.Exchange,
		MarketType:  event.MarketType,
		Symbol:      event.Symbol,
		Type:        "funding_anomaly",
		Severity:    "warning",
		Title:       fmt.Sprintf("⚠️ %s Funding 异常", event.Symbol),
		Message:     fmt.Sprintf("当前 Funding: %.4f%%", rate),
		EventID:     event.ID,
		TriggerKey:  event.TriggerBucket("funding_anomaly"),
		TriggerTime: event.EventTime,
		CreatedAt:   time.Now().UTC(),
	}, true
}
