package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
)

type FundingFetcher struct {
	baseURL string
	symbols []string
	bus     *eventbus.Bus
	client  *http.Client
	now     func() time.Time
}

func NewFundingFetcher(baseURL string, symbols []string, bus *eventbus.Bus) FundingFetcher {
	return FundingFetcher{
		baseURL: baseURL,
		symbols: append([]string(nil), symbols...),
		bus:     bus,
		client:  &http.Client{Timeout: 10 * time.Second},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (f FundingFetcher) Fetch(ctx context.Context) error {
	rates, err := f.fetchRates(ctx)
	if err != nil {
		return err
	}
	for _, item := range rates {
		event := NormalizeFunding(item.Symbol, item.LastFundingRate*100, item.Time)
		if err := f.bus.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type fundingRatePayload struct {
	Symbol          string `json:"symbol"`
	LastFundingRate string `json:"lastFundingRate"`
	Time            int64  `json:"time"`
}

type fundingRate struct {
	Symbol          string
	LastFundingRate float64
	Time            int64
}

func (f FundingFetcher) fetchRates(ctx context.Context) ([]fundingRate, error) {
	client := f.client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	reqURL, err := url.Parse(f.baseURL)
	if err != nil {
		return nil, err
	}
	reqURL.Path = "/fapi/v1/premiumIndex"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("funding request failed: %s", resp.Status)
	}

	var payload []fundingRatePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	filter := make(map[string]struct{}, len(f.symbols))
	for _, symbol := range f.symbols {
		filter[symbol] = struct{}{}
	}

	out := make([]fundingRate, 0, len(f.symbols))
	for _, item := range payload {
		if _, ok := filter[item.Symbol]; !ok {
			continue
		}
		rate, err := strconv.ParseFloat(item.LastFundingRate, 64)
		if err != nil {
			return nil, err
		}
		ts := item.Time
		if ts == 0 {
			ts = f.now().UnixMilli()
		}
		out = append(out, fundingRate{
			Symbol:          item.Symbol,
			LastFundingRate: rate,
			Time:            ts,
		})
	}
	return out, nil
}
