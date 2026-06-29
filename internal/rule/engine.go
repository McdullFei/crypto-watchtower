package rule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/notifier"
)

type Config struct {
	LargeTradeThreshold       float64
	LargeTradeWindowThreshold float64
	LargeTradeWindowSec       int
	LiquidationThreshold      float64
	FundingAbsThreshold       float64
}

type Engine struct {
	mu        sync.RWMutex
	config    Config
	overrides map[ruleKey]ruleOverride
	windows   map[windowKey]tradeWindowState
}

type ruleKey struct {
	exchange string
	symbol   string
	ruleType string
}

type ruleOverride struct {
	threshold float64
	enabled   bool
}

type windowKey struct {
	exchange   string
	marketType string
	symbol     string
}

type tradeWindowEvent struct {
	eventID   string
	notional  float64
	eventTime time.Time
}

type tradeWindowState struct {
	events []tradeWindowEvent
	total  float64
}

func NewEngine(cfg Config) *Engine {
	return &Engine{
		config:    cfg,
		overrides: make(map[ruleKey]ruleOverride),
		windows:   make(map[windowKey]tradeWindowState),
	}
}

func (e *Engine) Evaluate(event model.MarketEvent) []model.Alert {
	var alerts []model.Alert
	if alert, ok := e.largeTradeRule(event).Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := e.liquidationRule(event).Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := e.fundingRule(event).Evaluate(event); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := e.windowLargeTradeRule(event); ok {
		alerts = append(alerts, alert)
	}
	return alerts
}

func (e *Engine) LoadRules(rules []model.AlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.overrides = make(map[ruleKey]ruleOverride, len(rules))
	for _, rule := range rules {
		e.overrides[toRuleKey(rule)] = ruleOverride{
			threshold: rule.Threshold,
			enabled:   rule.Enabled,
		}
	}
}

func (e *Engine) ApplyRule(rule model.AlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.overrides[toRuleKey(rule)] = ruleOverride{
		threshold: rule.Threshold,
		enabled:   rule.Enabled,
	}
}

type Sender interface {
	Send(context.Context, model.Alert) error
}

// NamedSender wraps one notification sender with log metadata.
//
// Author: __AUTHOR__
// Date: 2026-06-29
type NamedSender interface {
	Name() string
	Target() string
	Send(context.Context, model.Alert) error
}

type namedSender struct {
	name   string
	target string
	sender Sender
}

// NewNamedSender adds channel metadata to a sender without changing its send behavior.
//
// Author: __AUTHOR__
// Date: 2026-06-29
// @param name Notification channel name.
// @param target Notification target identifier.
// @param sender Sender implementation.
// @returns Channel-aware sender.
func NewNamedSender(name string, target string, sender Sender) NamedSender {
	return namedSender{name: name, target: target, sender: sender}
}

func (s namedSender) Name() string {
	return s.name
}

func (s namedSender) Target() string {
	return s.target
}

func (s namedSender) Send(ctx context.Context, alert model.Alert) error {
	return s.sender.Send(ctx, alert)
}

type Evaluator interface {
	Evaluate(model.MarketEvent) []model.Alert
}

type pipelineRepositories interface {
	InsertMarketEvent(context.Context, model.MarketEvent) error
	InsertAlert(context.Context, model.Alert) error
	InsertNotificationLog(context.Context, model.NotificationLog) error
}

type Pipeline struct {
	engine  Evaluator
	repos   pipelineRepositories
	redis   redis.UniversalClient
	senders []NamedSender
}

func NewPipeline(engine Evaluator, repos pipelineRepositories, redis redis.UniversalClient, senders ...NamedSender) Pipeline {
	return Pipeline{engine: engine, repos: repos, redis: redis, senders: senders}
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

		if err := p.repos.InsertMarketEvent(ctx, event); err != nil {
			return fmt.Errorf("insert market event: %w", err)
		}
		if err := p.repos.InsertAlert(ctx, alert); err != nil {
			return fmt.Errorf("insert alert: %w", err)
		}
		for _, sender := range p.senders {
			sendErr := sender.Send(ctx, alert)
			logStatus := "sent"
			logMessage := ""
			if sendErr != nil {
				logStatus = "failed"
				logMessage = sendErr.Error()
			}
			if err := p.repos.InsertNotificationLog(ctx, model.NotificationLog{
				AlertID:      alert.ID,
				Channel:      sender.Name(),
				Target:       sender.Target(),
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

func (e *Engine) largeTradeRule(event model.MarketEvent) LargeTradeRule {
	threshold, enabled := e.thresholdFor(event.Exchange, event.Symbol, "large_trade", e.config.LargeTradeThreshold)
	if !enabled {
		return LargeTradeRule{Threshold: 1e18}
	}
	return LargeTradeRule{Threshold: threshold}
}

func (e *Engine) liquidationRule(event model.MarketEvent) LiquidationRule {
	threshold, enabled := e.thresholdFor(event.Exchange, event.Symbol, "liquidation", e.config.LiquidationThreshold)
	if !enabled {
		return LiquidationRule{Threshold: 1e18}
	}
	return LiquidationRule{Threshold: threshold}
}

func (e *Engine) fundingRule(event model.MarketEvent) FundingRule {
	threshold, enabled := e.thresholdFor(event.Exchange, event.Symbol, "funding_anomaly", e.config.FundingAbsThreshold)
	if !enabled {
		return FundingRule{AbsThreshold: 1e18}
	}
	return FundingRule{AbsThreshold: threshold}
}

func (e *Engine) windowLargeTradeRule(event model.MarketEvent) (model.Alert, bool) {
	if event.EventType != "agg_trade" || event.Notional <= 0 {
		return model.Alert{}, false
	}
	windowSec := e.config.LargeTradeWindowSec
	if windowSec <= 0 {
		windowSec = 60
	}
	threshold, enabled := e.thresholdFor(event.Exchange, event.Symbol, "large_trade_window", e.config.LargeTradeWindowThreshold)
	if !enabled || threshold <= 0 {
		return model.Alert{}, false
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	key := windowKey{
		exchange:   event.Exchange,
		marketType: event.MarketType,
		symbol:     event.Symbol,
	}
	state := e.windows[key]
	cutoff := event.EventTime.Add(-time.Duration(windowSec) * time.Second)

	kept := state.events[:0]
	total := 0.0
	for _, item := range state.events {
		if item.eventTime.Before(cutoff) {
			continue
		}
		kept = append(kept, item)
		total += item.notional
	}

	previousTotal := total
	kept = append(kept, tradeWindowEvent{
		eventID:   event.ID,
		notional:  event.Notional,
		eventTime: event.EventTime,
	})
	total += event.Notional
	state.events = kept
	state.total = total
	e.windows[key] = state

	if previousTotal >= threshold || total < threshold {
		return model.Alert{}, false
	}
	return model.Alert{
		ID:          event.ID + "-large-trade-window",
		Exchange:    event.Exchange,
		MarketType:  event.MarketType,
		Symbol:      event.Symbol,
		Type:        "large_trade_window",
		Severity:    "warning",
		Title:       fmt.Sprintf("📈 %s 60s 累计成交额异动", event.Symbol),
		Message:     fmt.Sprintf("60s累计成交额: %.2f USDT\n阈值: %.2f USDT\n市场: %s", total, threshold, event.MarketType),
		EventID:     event.ID,
		TriggerKey:  event.TriggerBucket("large_trade_window"),
		TriggerTime: event.EventTime,
		CreatedAt:   time.Now().UTC(),
	}, true
}

func (e *Engine) thresholdFor(exchange, symbol, ruleType string, fallback float64) (float64, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	override, ok := e.overrides[ruleKey{
		exchange: exchange,
		symbol:   symbol,
		ruleType: ruleType,
	}]
	if ok {
		return override.threshold, override.enabled
	}
	return fallback, fallback > 0
}

func toRuleKey(rule model.AlertRule) ruleKey {
	return ruleKey{
		exchange: rule.Exchange,
		symbol:   rule.Symbol,
		ruleType: rule.RuleType,
	}
}
