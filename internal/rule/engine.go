package rule

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/notifier"
	"github.com/renfei198727/crypto-watchtower/internal/storage"
)

type Config struct {
	LargeTradeThreshold  float64
	LiquidationThreshold float64
	FundingAbsThreshold  float64
}

type Engine struct {
	largeTrade  LargeTradeRule
	liquidation LiquidationRule
	funding     FundingRule
}

func NewEngine(cfg Config) Engine {
	return Engine{
		largeTrade:  LargeTradeRule{Threshold: cfg.LargeTradeThreshold},
		liquidation: LiquidationRule{Threshold: cfg.LiquidationThreshold},
		funding:     FundingRule{AbsThreshold: cfg.FundingAbsThreshold},
	}
}

func (e Engine) Evaluate(event model.MarketEvent) []model.Alert {
	var alerts []model.Alert
	if alert, ok := e.largeTrade.Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := e.liquidation.Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := e.funding.Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	return alerts
}

type Sender interface {
	Send(context.Context, model.Alert) error
}

type Pipeline struct {
	engine Engine
	repos  *storage.Repositories
	redis  redis.UniversalClient
	send   Sender
}

func NewPipeline(engine Engine, repos *storage.Repositories, redis redis.UniversalClient, send Sender) Pipeline {
	return Pipeline{engine: engine, repos: repos, redis: redis, send: send}
}

func (p Pipeline) HandleEvent(ctx context.Context, event model.MarketEvent) error {
	alerts := p.engine.Evaluate(event)
	for _, alert := range alerts {
		ok, err := p.allowAlert(ctx, alert)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		if err := p.repos.MarketEvents.Insert(ctx, event); err != nil {
			return fmt.Errorf("insert market event: %w", err)
		}
		if err := p.repos.Alerts.Insert(ctx, alert); err != nil {
			return fmt.Errorf("insert alert: %w", err)
		}
		sendErr := p.send.Send(ctx, alert)
		logStatus := "sent"
		logMessage := ""
		if sendErr != nil {
			logStatus = "failed"
			logMessage = sendErr.Error()
		}
		if err := p.repos.NotificationLogs.Insert(ctx, model.NotificationLog{
			AlertID:      alert.ID,
			Channel:      "telegram",
			Target:       "default",
			Status:       logStatus,
			ErrorMessage: logMessage,
			CreatedAt:    time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("insert notification log: %w", err)
		}
		if sendErr != nil {
			return sendErr
		}
	}
	return nil
}

func (p Pipeline) allowAlert(ctx context.Context, alert model.Alert) (bool, error) {
	if p.redis == nil {
		return true, nil
	}
	dedupeKey := "dedupe:alert:" + alert.TriggerKey + ":" + alert.EventID
	limitedKey := "rate_limit:alert:" + alert.TriggerKey

	set, err := p.redis.SetNX(ctx, dedupeKey, "1", 120*time.Second).Result()
	if err != nil || !set {
		return set, err
	}
	set, err = p.redis.SetNX(ctx, limitedKey, "1", 60*time.Second).Result()
	if err != nil || !set {
		return set, err
	}
	return true, nil
}

var _ Sender = notifier.TelegramNotifier{}
