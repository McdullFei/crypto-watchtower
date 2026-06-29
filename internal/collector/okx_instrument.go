package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// OKXInstrument stores the metadata needed to normalize OKX market events.
//
// Author: monsterfei
// Date: 2026-06-29
type OKXInstrument struct {
	InstID    string
	InstType  string
	CtVal     float64
	CtValCcy  string
	SettleCcy string
}

// OKXInstrumentStore provides bounded lookup and notional helpers for OKX events.
//
// Author: monsterfei
// Date: 2026-06-29
type OKXInstrumentStore struct {
	byID map[string]OKXInstrument
}

// NewOKXInstrumentStore builds an instrument store keyed by OKX instrument ID.
//
// Author: monsterfei
// Date: 2026-06-29
func NewOKXInstrumentStore(instruments []OKXInstrument) OKXInstrumentStore {
	byID := make(map[string]OKXInstrument, len(instruments))
	for _, instrument := range instruments {
		if instrument.InstID == "" {
			continue
		}
		byID[instrument.InstID] = instrument
	}
	return OKXInstrumentStore{byID: byID}
}

// Lookup returns instrument metadata for an OKX instrument ID.
//
// Author: monsterfei
// Date: 2026-06-29
func (s OKXInstrumentStore) Lookup(instID string) (OKXInstrument, bool) {
	instrument, ok := s.byID[instID]
	return instrument, ok
}

// Notional calculates USDT notional for supported OKX spot and linear swap events.
//
// Author: monsterfei
// Date: 2026-06-29
func (s OKXInstrumentStore) Notional(instID string, price float64, size float64) (float64, error) {
	instrument, ok := s.Lookup(instID)
	if !ok {
		return 0, fmt.Errorf("okx instrument %s is not loaded", instID)
	}
	switch instrument.InstType {
	case "SPOT":
		return price * size, nil
	case "SWAP", "FUTURES":
		if instrument.SettleCcy != "USDT" {
			return 0, fmt.Errorf("okx instrument %s settle currency %s is not supported", instID, instrument.SettleCcy)
		}
		if instrument.CtVal <= 0 {
			return 0, fmt.Errorf("okx instrument %s ctVal is required", instID)
		}
		return price * size * instrument.CtVal, nil
	default:
		return 0, fmt.Errorf("okx instrument type %s is not supported", instrument.InstType)
	}
}

// OKXSpotInstID maps a compact USDT symbol to an OKX spot instrument ID.
//
// Author: monsterfei
// Date: 2026-06-29
func OKXSpotInstID(symbol string) string {
	base := strings.TrimSuffix(strings.ToUpper(symbol), "USDT")
	if base == strings.ToUpper(symbol) || base == "" {
		return strings.ToUpper(symbol)
	}
	return base + "-USDT"
}

// OKXSwapInstID maps a compact USDT symbol to an OKX USDT swap instrument ID.
//
// Author: monsterfei
// Date: 2026-06-29
func OKXSwapInstID(symbol string) string {
	spot := OKXSpotInstID(symbol)
	if !strings.Contains(spot, "-") {
		return spot
	}
	return spot + "-SWAP"
}

// OKXInstrumentFetcher loads public OKX instrument metadata.
//
// Author: monsterfei
// Date: 2026-06-29
type OKXInstrumentFetcher struct {
	baseURL string
	client  *http.Client
}

// NewOKXInstrumentFetcher creates a fetcher for OKX public instruments.
//
// Author: monsterfei
// Date: 2026-06-29
func NewOKXInstrumentFetcher(baseURL string, client *http.Client) OKXInstrumentFetcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return OKXInstrumentFetcher{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

// Fetch retrieves OKX instruments for one instrument type.
//
// Author: monsterfei
// Date: 2026-06-29
func (f OKXInstrumentFetcher) Fetch(ctx context.Context, instType string) ([]OKXInstrument, error) {
	if f.baseURL == "" {
		return nil, errors.New("okx rest base URL is required")
	}
	endpoint, err := url.Parse(f.baseURL + "/api/v5/public/instruments")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("instType", instType)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("okx instruments status %d", resp.StatusCode)
	}

	var payload okxInstrumentResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Code != "0" {
		return nil, fmt.Errorf("okx instruments code %s: %s", payload.Code, payload.Message)
	}
	instruments := make([]OKXInstrument, 0, len(payload.Data))
	for _, item := range payload.Data {
		ctVal := 0.0
		if item.CtVal != "" {
			parsed, err := strconv.ParseFloat(item.CtVal, 64)
			if err != nil {
				return nil, err
			}
			ctVal = parsed
		}
		instruments = append(instruments, OKXInstrument{
			InstID:    item.InstID,
			InstType:  item.InstType,
			CtVal:     ctVal,
			CtValCcy:  item.CtValCcy,
			SettleCcy: item.SettleCcy,
		})
	}
	return instruments, nil
}

type okxInstrumentResponse struct {
	Code    string                `json:"code"`
	Message string                `json:"msg"`
	Data    []okxInstrumentRecord `json:"data"`
}

type okxInstrumentRecord struct {
	InstID    string `json:"instId"`
	InstType  string `json:"instType"`
	CtVal     string `json:"ctVal"`
	CtValCcy  string `json:"ctValCcy"`
	SettleCcy string `json:"settleCcy"`
}
