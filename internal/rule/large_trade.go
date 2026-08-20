package rule

import (
	"fmt"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type LargeTradeRule struct {
	Threshold float64
}

// Evaluate returns a large-trade alert when one aggregate trade exceeds the configured notional threshold.
//
// Author: monsterfei
// Date: 2026-07-02
// @param event Market event to evaluate.
// @returns Alert and whether the rule matched.
func (r LargeTradeRule) Evaluate(event model.MarketEvent) (model.Alert, bool) {
	if event.EventType != "agg_trade" || event.Notional < r.Threshold {
		return model.Alert{}, false
	}
	return model.Alert{
		ID:         event.ID + "-large-trade",
		Exchange:   event.Exchange,
		MarketType: event.MarketType,
		Symbol:     event.Symbol,
		Type:       "large_trade",
		Severity:   "warning",
		Title:      fmt.Sprintf("🚨 %s 大额%s", event.Symbol, event.Side),
		Message: fmt.Sprintf(
			"交易所: %s\n交易对: %s\n规则: large_trade\n阈值: %.2f USDT\n价格: %.4f\n成交额: %.2f USDT\n市场: %s",
			event.Exchange,
			event.Symbol,
			r.Threshold,
			event.Price,
			event.Notional,
			event.MarketType,
		),
		EventID:     event.ID,
		TriggerKey:  event.TriggerBucket("large_trade"),
		TriggerTime: event.EventTime,
		CreatedAt:   time.Now().UTC(),
	}, true
}
