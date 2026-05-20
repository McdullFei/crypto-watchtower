package rule

import (
	"fmt"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type LiquidationRule struct {
	Threshold float64
}

func (r LiquidationRule) Evaluate(event model.MarketEvent) (model.Alert, bool) {
	if event.EventType != "liquidation" || event.Notional < r.Threshold {
		return model.Alert{}, false
	}
	return model.Alert{
		ID:          event.ID + "-liquidation",
		Exchange:    event.Exchange,
		MarketType:  event.MarketType,
		Symbol:      event.Symbol,
		Type:        "liquidation",
		Severity:    "critical",
		Title:       fmt.Sprintf("💥 %s 大额爆仓", event.Symbol),
		Message:     fmt.Sprintf("方向: %s\n金额: %.2f USDT\n价格: %.4f", event.Side, event.Notional, event.Price),
		EventID:     event.ID,
		TriggerKey:  event.TriggerBucket("liquidation"),
		TriggerTime: event.EventTime,
		CreatedAt:   time.Now().UTC(),
	}, true
}
