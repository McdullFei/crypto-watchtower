package collector

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

const (
	MarketTypeSpot    = "spot"
	MarketTypeFutures = "futures"
)

type aggTradePayload struct {
	EventType string `json:"e"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"a"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	Maker     bool   `json:"m"`
	EventTime int64  `json:"T"`
}

type liquidationEnvelope struct {
	Order liquidationPayload `json:"o"`
}

type liquidationPayload struct {
	Symbol    string `json:"s"`
	Side      string `json:"S"`
	AvgPrice  string `json:"ap"`
	Quantity  string `json:"q"`
	EventTime int64  `json:"T"`
}

func NormalizeAggTrade(raw []byte, marketType string) (model.MarketEvent, error) {
	var payload aggTradePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.MarketEvent{}, err
	}

	price, err := strconv.ParseFloat(payload.Price, 64)
	if err != nil {
		return model.MarketEvent{}, err
	}
	qty, err := strconv.ParseFloat(payload.Quantity, 64)
	if err != nil {
		return model.MarketEvent{}, err
	}

	side := "Aggressive Buy"
	if payload.Maker {
		side = "Aggressive Sell"
	}

	return model.MarketEvent{
		ID:         fmt.Sprintf("binance-%s-agg-%d", marketType, payload.TradeID),
		Exchange:   "binance",
		MarketType: marketType,
		Symbol:     payload.Symbol,
		EventType:  "agg_trade",
		Side:       side,
		Price:      price,
		Quantity:   qty,
		Notional:   price * qty,
		RawPayload: raw,
		EventTime:  time.UnixMilli(payload.EventTime).UTC(),
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func NormalizeLiquidation(raw []byte) (model.MarketEvent, error) {
	var envelope liquidationEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return model.MarketEvent{}, err
	}
	price, err := strconv.ParseFloat(envelope.Order.AvgPrice, 64)
	if err != nil {
		return model.MarketEvent{}, err
	}
	qty, err := strconv.ParseFloat(envelope.Order.Quantity, 64)
	if err != nil {
		return model.MarketEvent{}, err
	}
	side := "Short Liquidation"
	if envelope.Order.Side == "SELL" {
		side = "Long Liquidation"
	}
	return model.MarketEvent{
		ID:         fmt.Sprintf("binance-futures-force-%s-%d", envelope.Order.Symbol, envelope.Order.EventTime),
		Exchange:   "binance",
		MarketType: MarketTypeFutures,
		Symbol:     envelope.Order.Symbol,
		EventType:  "liquidation",
		Side:       side,
		Price:      price,
		Quantity:   qty,
		Notional:   price * qty,
		RawPayload: raw,
		EventTime:  time.UnixMilli(envelope.Order.EventTime).UTC(),
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func NormalizeFunding(symbol string, rate float64, eventMillis int64) model.MarketEvent {
	return model.MarketEvent{
		ID:         fmt.Sprintf("binance-futures-funding-%s-%d", symbol, eventMillis),
		Exchange:   "binance",
		MarketType: MarketTypeFutures,
		Symbol:     symbol,
		EventType:  "funding",
		Metadata: map[string]any{
			"funding_rate": rate,
		},
		EventTime: time.UnixMilli(eventMillis).UTC(),
		CreatedAt: time.Now().UTC(),
	}
}
