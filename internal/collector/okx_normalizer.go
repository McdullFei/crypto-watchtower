package collector

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// NormalizeOKXTrades converts OKX trades channel payloads into market events.
//
// Author: monsterfei
// Date: 2026-06-29
func NormalizeOKXTrades(raw []byte, store OKXInstrumentStore) ([]model.MarketEvent, error) {
	var payload okxTradesMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	events := make([]model.MarketEvent, 0, len(payload.Data))
	for _, item := range payload.Data {
		price, size, err := parseOKXPriceSize(item.Price, item.Size)
		if err != nil {
			return nil, err
		}
		notional, err := store.Notional(item.InstID, price, size)
		if err != nil {
			return nil, err
		}
		marketType := OKXMarketType(item.InstID)
		events = append(events, model.MarketEvent{
			ID:         fmt.Sprintf("okx-%s-trades-%s-%s", marketType, item.InstID, item.TradeID),
			Exchange:   "okx",
			MarketType: marketType,
			Symbol:     OKXSymbolFromInstID(item.InstID),
			EventType:  "agg_trade",
			Side:       okxTradeSide(item.Side),
			Price:      price,
			Quantity:   size,
			Notional:   notional,
			Metadata: map[string]any{
				"native_inst_id": item.InstID,
				"count":          item.Count,
				"source":         item.Source,
				"seq_id":         item.SeqID,
			},
			RawPayload: raw,
			EventTime:  time.UnixMilli(item.Timestamp).UTC(),
			CreatedAt:  time.Now().UTC(),
		})
	}
	return events, nil
}

// NormalizeOKXLiquidations converts OKX liquidation-orders payloads into events.
//
// Author: monsterfei
// Date: 2026-06-29
func NormalizeOKXLiquidations(raw []byte, store OKXInstrumentStore) ([]model.MarketEvent, error) {
	var payload okxLiquidationMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	var events []model.MarketEvent
	for _, item := range payload.Data {
		for _, detail := range item.Details {
			price, size, err := parseOKXPriceSize(detail.BankruptcyPrice, detail.Size)
			if err != nil {
				return nil, err
			}
			notional, err := store.Notional(item.InstID, price, size)
			if err != nil {
				return nil, err
			}
			events = append(events, model.MarketEvent{
				ID:         fmt.Sprintf("okx-%s-liquidation-%s-%d", OKXMarketType(item.InstID), item.InstID, detail.Timestamp),
				Exchange:   "okx",
				MarketType: OKXMarketType(item.InstID),
				Symbol:     OKXSymbolFromInstID(item.InstID),
				EventType:  "liquidation",
				Side:       okxLiquidationSide(detail.PositionSide, detail.Side),
				Price:      price,
				Quantity:   size,
				Notional:   notional,
				Metadata: map[string]any{
					"native_inst_id": item.InstID,
					"inst_family":    item.InstFamily,
					"inst_type":      item.InstType,
					"pos_side":       detail.PositionSide,
					"bk_loss":        detail.BankruptcyLoss,
				},
				RawPayload: raw,
				EventTime:  time.UnixMilli(detail.Timestamp).UTC(),
				CreatedAt:  time.Now().UTC(),
			})
		}
	}
	return events, nil
}

// NormalizeOKXFunding converts OKX funding-rate payloads into funding events.
//
// Author: monsterfei
// Date: 2026-06-29
func NormalizeOKXFunding(raw []byte) ([]model.MarketEvent, error) {
	var payload okxFundingMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	events := make([]model.MarketEvent, 0, len(payload.Data))
	for _, item := range payload.Data {
		rate, err := strconv.ParseFloat(item.FundingRate, 64)
		if err != nil {
			return nil, err
		}
		events = append(events, model.MarketEvent{
			ID:         fmt.Sprintf("okx-%s-funding-%s-%d", OKXMarketType(item.InstID), item.InstID, item.FundingTime),
			Exchange:   "okx",
			MarketType: OKXMarketType(item.InstID),
			Symbol:     OKXSymbolFromInstID(item.InstID),
			EventType:  "funding",
			Metadata: map[string]any{
				"funding_rate":      rate * 100,
				"native_inst_id":    item.InstID,
				"next_funding_time": item.NextFundingTime,
				"sett_state":        item.SettlementState,
				"sett_funding_rate": item.SettlementFundingRate,
				"formula_type":      item.FormulaType,
				"method":            item.Method,
			},
			RawPayload: raw,
			EventTime:  time.UnixMilli(item.FundingTime).UTC(),
			CreatedAt:  time.Now().UTC(),
		})
	}
	return events, nil
}

// OKXSymbolFromInstID maps OKX native instrument IDs to project symbols.
//
// Author: monsterfei
// Date: 2026-06-29
func OKXSymbolFromInstID(instID string) string {
	parts := strings.Split(strings.ToUpper(instID), "-")
	if len(parts) < 2 {
		return strings.ToUpper(instID)
	}
	return parts[0] + parts[1]
}

// OKXMarketType maps OKX instrument IDs to the project market type.
//
// Author: monsterfei
// Date: 2026-06-29
func OKXMarketType(instID string) string {
	if strings.HasSuffix(strings.ToUpper(instID), "-SWAP") {
		return MarketTypeFutures
	}
	return MarketTypeSpot
}

func parseOKXPriceSize(priceRaw string, sizeRaw string) (float64, float64, error) {
	price, err := strconv.ParseFloat(priceRaw, 64)
	if err != nil {
		return 0, 0, err
	}
	size, err := strconv.ParseFloat(sizeRaw, 64)
	if err != nil {
		return 0, 0, err
	}
	return price, size, nil
}

func okxTradeSide(side string) string {
	if side == "sell" {
		return "Aggressive Sell"
	}
	return "Aggressive Buy"
}

func okxLiquidationSide(posSide string, side string) string {
	switch posSide {
	case "long":
		return "Long Liquidation"
	case "short":
		return "Short Liquidation"
	}
	if side == "sell" {
		return "Long Liquidation"
	}
	return "Short Liquidation"
}

type okxTradesMessage struct {
	Data []okxTradeRecord `json:"data"`
}

type okxTradeRecord struct {
	InstID    string `json:"instId"`
	TradeID   string `json:"tradeId"`
	Price     string `json:"px"`
	Size      string `json:"sz"`
	Side      string `json:"side"`
	Timestamp int64  `json:"ts,string"`
	Count     string `json:"count"`
	Source    string `json:"source"`
	SeqID     int64  `json:"seqId"`
}

type okxLiquidationMessage struct {
	Data []okxLiquidationRecord `json:"data"`
}

type okxLiquidationRecord struct {
	InstFamily string                 `json:"instFamily"`
	InstID     string                 `json:"instId"`
	InstType   string                 `json:"instType"`
	Details    []okxLiquidationDetail `json:"details"`
}

type okxLiquidationDetail struct {
	BankruptcyLoss  string `json:"bkLoss"`
	BankruptcyPrice string `json:"bkPx"`
	PositionSide    string `json:"posSide"`
	Side            string `json:"side"`
	Size            string `json:"sz"`
	Timestamp       int64  `json:"ts,string"`
}

type okxFundingMessage struct {
	Data []okxFundingRecord `json:"data"`
}

type okxFundingRecord struct {
	FormulaType           string `json:"formulaType"`
	FundingRate           string `json:"fundingRate"`
	FundingTime           int64  `json:"fundingTime,string"`
	InstID                string `json:"instId"`
	InstType              string `json:"instType"`
	Method                string `json:"method"`
	NextFundingTime       int64  `json:"nextFundingTime,string"`
	SettlementFundingRate string `json:"settFundingRate"`
	SettlementState       string `json:"settState"`
}
