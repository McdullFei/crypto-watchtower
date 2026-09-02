//go:build integration

package rule

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// TestUpdateRedisUserWindowIsAtomic verifies concurrent updates and cutoff pruning against real Redis.
//
// Author: monsterfei
// Date: 2026-09-02
// @param t Testing context.
func TestUpdateRedisUserWindowIsAtomic(t *testing.T) {
	if os.Getenv("CW_INTEGRATION_TESTS") != "1" {
		t.Skip("set CW_INTEGRATION_TESTS=1 to run the real Redis integration test")
	}
	redisAddr := os.Getenv("CW_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddr, Password: os.Getenv("CW_REDIS_PASSWORD")})
	defer client.Close()

	ctx := context.Background()
	pipeline := Pipeline{redis: client}
	userID := time.Now().UnixNano()
	ruleID := int64(902)
	eventTime := time.Date(2026, 9, 2, 5, 0, 0, 0, time.UTC)
	key := fmt.Sprintf("window:user_rule:%d:%d:binance:spot:ATOMICUSDT", userID, ruleID)
	defer client.Del(ctx, key)

	type windowResult struct {
		previous float64
		total    float64
		err      error
	}
	results := make(chan windowResult, 2)
	for index := 0; index < 2; index++ {
		go func(index int) {
			event := model.MarketEvent{
				ID:         fmt.Sprintf("redis-atomic-%d", index),
				Exchange:   "binance",
				MarketType: "spot",
				Symbol:     "ATOMICUSDT",
				Notional:   1000,
				EventTime:  eventTime,
			}
			previous, total, err := pipeline.updateRedisUserWindow(ctx, event, userID, ruleID, 60)
			results <- windowResult{previous: previous, total: total, err: err}
		}(index)
	}

	actual := make([]windowResult, 0, 2)
	for index := 0; index < 2; index++ {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent Redis window update: %v", result.err)
		}
		actual = append(actual, result)
	}
	sort.Slice(actual, func(i, j int) bool { return actual[i].previous < actual[j].previous })
	if actual[0].previous != 0 || actual[0].total != 1000 || actual[1].previous != 1000 || actual[1].total != 2000 {
		t.Fatalf("unexpected atomic snapshots: %+v", actual)
	}
	if count, err := client.ZCard(ctx, key).Result(); err != nil || count != 2 {
		t.Fatalf("expected two Redis window members, count=%d err=%v", count, err)
	}
	if ttl, err := client.TTL(ctx, key).Result(); err != nil || ttl <= 0 {
		t.Fatalf("expected positive Redis window TTL, ttl=%v err=%v", ttl, err)
	}

	cutoffEvent := model.MarketEvent{
		ID:         "redis-atomic-cutoff",
		Exchange:   "binance",
		MarketType: "spot",
		Symbol:     "ATOMICUSDT",
		Notional:   500,
		EventTime:  eventTime.Add(60 * time.Second),
	}
	previous, total, err := pipeline.updateRedisUserWindow(ctx, cutoffEvent, userID, ruleID, 60)
	if err != nil {
		t.Fatalf("exact-cutoff Redis window update: %v", err)
	}
	if previous != 0 || total != 500 {
		t.Fatalf("expected exact-cutoff members to expire, previous=%v total=%v", previous, total)
	}
	if count, err := client.ZCard(ctx, key).Result(); err != nil || count != 1 {
		t.Fatalf("expected only the cutoff event to remain, count=%d err=%v", count, err)
	}
}
