package rule

import (
	"context"
	"fmt"
	"strconv"
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

// userWindowKey isolates user-rule window state from global system rule state.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type userWindowKey struct {
	userID     int64
	ruleID     int64
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
// Author: monsterfei
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
// Author: monsterfei
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

// UserRuleTarget combines one user-scoped rule with its owning account.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type UserRuleTarget = model.UserRuleTarget

// UserRuleRepository defines bounded user-rule lookup for fanout.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type UserRuleRepository interface {
	ListActiveUserRulesForEvent(context.Context, model.MarketEvent, int) ([]UserRuleTarget, error)
}

// UserAlertSender sends alerts to a user-specific notification target.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type UserAlertSender interface {
	SendTo(context.Context, string, model.Alert) error
}

type Pipeline struct {
	engine          Evaluator
	repos           pipelineRepositories
	redis           redis.UniversalClient
	senders         []NamedSender
	userRules       UserRuleRepository
	userSender      UserAlertSender
	userSenderName  string
	userRuleMaxScan int
	userWindowMu    *sync.Mutex
	userWindows     map[userWindowKey]tradeWindowState
}

func NewPipeline(engine Evaluator, repos pipelineRepositories, redis redis.UniversalClient, senders ...NamedSender) Pipeline {
	return Pipeline{engine: engine, repos: repos, redis: redis, senders: senders}
}

// WithUserFanout enables user-scoped rule evaluation and targeted notification delivery.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param userRules Bounded user rule repository.
// @param senderName Notification channel name for logs.
// @param userSender Targeted user notification sender.
// @returns Pipeline copy with user fanout enabled.
func (p Pipeline) WithUserFanout(userRules UserRuleRepository, senderName string, userSender UserAlertSender) Pipeline {
	p.userRules = userRules
	p.userSender = userSender
	p.userSenderName = senderName
	p.userRuleMaxScan = 200
	p.userWindowMu = &sync.Mutex{}
	p.userWindows = make(map[userWindowKey]tradeWindowState)
	return p
}

// HandleEvent evaluates one market event and records alert delivery results for every configured sender.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param event Market event to evaluate.
// @returns Error when persistence, deduplication, or at least one sender fails.
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
		var firstSendErr error
		for _, sender := range p.senders {
			currentSendErr := sender.Send(ctx, alert)
			logStatus := "sent"
			logMessage := ""
			if currentSendErr != nil {
				logStatus = "failed"
				logMessage = currentSendErr.Error()
				if firstSendErr == nil {
					firstSendErr = currentSendErr
				}
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
		}
		if firstSendErr != nil {
			return firstSendErr
		}
	}
	if err := p.handleUserRules(ctx, event); err != nil {
		return err
	}
	return nil
}

// handleUserRules evaluates bounded user-scoped rules and sends alerts to bound users.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param event Market event to evaluate.
// @returns Error when lookup, persistence, or delivery fails.
func (p Pipeline) handleUserRules(ctx context.Context, event model.MarketEvent) error {
	if p.userRules == nil || p.userSender == nil {
		return nil
	}
	limit := p.userRuleMaxScan
	if limit <= 0 {
		limit = 200
	}
	targets, err := p.userRules.ListActiveUserRulesForEvent(ctx, event, limit)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if target.User.Status == model.UserStatusDisabled || target.User.TelegramChatID == "" {
			continue
		}
		alert, ok, err := p.evaluateUserRule(ctx, event, target)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		allowed, err := p.allowUserAlert(ctx, alert, target.User.ID)
		if err != nil {
			return err
		}
		if !allowed {
			continue
		}
		if err := p.repos.InsertMarketEvent(ctx, event); err != nil {
			return fmt.Errorf("insert market event: %w", err)
		}
		if err := p.repos.InsertAlert(ctx, alert); err != nil {
			return fmt.Errorf("insert user alert: %w", err)
		}
		status := "sent"
		message := ""
		var sendErr error
		if !target.User.TelegramDeliveryEnabled {
			status = "disabled"
			message = "telegram delivery disabled"
		} else {
			sendErr = p.userSender.SendTo(ctx, target.User.TelegramChatID, alert)
			if sendErr != nil {
				status = "failed"
				message = sendErr.Error()
			}
		}
		userID := target.User.ID
		channel := p.userSenderName
		if channel == "" {
			channel = "telegram"
		}
		if err := p.repos.InsertNotificationLog(ctx, model.NotificationLog{
			UserID:       &userID,
			AlertID:      alert.ID,
			Channel:      channel,
			Target:       target.User.TelegramChatID,
			Status:       status,
			ErrorMessage: message,
			CreatedAt:    time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("insert user notification log: %w", err)
		}
		if sendErr != nil {
			return sendErr
		}
	}
	return nil
}

// evaluateUserRule evaluates one user-scoped rule without changing the global system engine.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param event Market event to evaluate.
// @param target User and rule context.
// @returns Alert, whether the rule matched, and state error.
func (p Pipeline) evaluateUserRule(ctx context.Context, event model.MarketEvent, target UserRuleTarget) (model.Alert, bool, error) {
	rule := target.Rule
	if !rule.Enabled {
		return model.Alert{}, false, nil
	}
	var alert model.Alert
	var ok bool
	switch rule.RuleType {
	case "large_trade":
		alert, ok = LargeTradeRule{Threshold: rule.Threshold}.Evaluate(event)
	case "large_trade_window":
		return p.evaluateUserWindowRule(ctx, event, target)
	case "liquidation":
		alert, ok = LiquidationRule{Threshold: rule.Threshold}.Evaluate(event)
	case "funding_anomaly":
		alert, ok = FundingRule{AbsThreshold: rule.Threshold}.Evaluate(event)
	default:
		return model.Alert{}, false, nil
	}
	if !ok {
		return model.Alert{}, false, nil
	}
	alert.ID = fmt.Sprintf("%s-user-%d-rule-%d", alert.ID, target.User.ID, rule.ID)
	alert.RuleID = fmt.Sprintf("user:%d", rule.ID)
	alert.TriggerKey = fmt.Sprintf("%s:user:%d:rule:%d", alert.TriggerKey, target.User.ID, rule.ID)
	return alert, true, nil
}

// evaluateUserWindowRule evaluates one user large-trade window rule with isolated bounded state.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param event Market event to evaluate.
// @param target User and rule context.
// @returns Alert, whether the rule matched, and state error.
func (p Pipeline) evaluateUserWindowRule(ctx context.Context, event model.MarketEvent, target UserRuleTarget) (model.Alert, bool, error) {
	rule := target.Rule
	if event.EventType != "agg_trade" || event.Notional <= 0 || rule.Threshold <= 0 {
		return model.Alert{}, false, nil
	}
	windowSec := rule.WindowSec
	if windowSec <= 0 {
		windowSec = 60
	}
	previousTotal, total, err := p.updateUserWindow(ctx, event, target.User.ID, rule.ID, windowSec)
	if err != nil {
		return model.Alert{}, false, err
	}
	if previousTotal >= rule.Threshold || total < rule.Threshold {
		return model.Alert{}, false, nil
	}
	alert := model.Alert{
		ID:          fmt.Sprintf("%s-large-trade-window-user-%d-rule-%d", event.ID, target.User.ID, rule.ID),
		RuleID:      fmt.Sprintf("user:%d", rule.ID),
		Exchange:    event.Exchange,
		MarketType:  event.MarketType,
		Symbol:      event.Symbol,
		Type:        "large_trade_window",
		Severity:    "warning",
		Title:       fmt.Sprintf("📈 %s %ds 累计成交额异动", event.Symbol, windowSec),
		Message:     fmt.Sprintf("%ds累计成交额: %.2f USDT\n阈值: %.2f USDT\n市场: %s", windowSec, total, rule.Threshold, event.MarketType),
		EventID:     event.ID,
		TriggerKey:  fmt.Sprintf("%s:user:%d:rule:%d", event.TriggerBucket("large_trade_window"), target.User.ID, rule.ID),
		TriggerTime: event.EventTime,
		CreatedAt:   time.Now().UTC(),
	}
	return alert, true, nil
}

// updateUserWindow stores one user window event and returns totals before and after the update.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param event Market event entering the window.
// @param userID User id for key isolation.
// @param ruleID Rule id for key isolation.
// @param windowSec Window size in seconds.
// @returns Previous total, updated total, and storage error.
func (p Pipeline) updateUserWindow(ctx context.Context, event model.MarketEvent, userID int64, ruleID int64, windowSec int) (float64, float64, error) {
	if p.redis != nil {
		return p.updateRedisUserWindow(ctx, event, userID, ruleID, windowSec)
	}
	return p.updateMemoryUserWindow(event, userID, ruleID, windowSec), p.userWindowTotal(event, userID, ruleID), nil
}

// updateMemoryUserWindow stores user window state in-process when Redis is not configured.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param event Market event entering the window.
// @param userID User id for key isolation.
// @param ruleID Rule id for key isolation.
// @param windowSec Window size in seconds.
// @returns Previous total before adding the event.
func (p Pipeline) updateMemoryUserWindow(event model.MarketEvent, userID int64, ruleID int64, windowSec int) float64 {
	if p.userWindowMu == nil || p.userWindows == nil {
		return 0
	}
	p.userWindowMu.Lock()
	defer p.userWindowMu.Unlock()

	key := userWindowKey{
		userID:     userID,
		ruleID:     ruleID,
		exchange:   event.Exchange,
		marketType: event.MarketType,
		symbol:     event.Symbol,
	}
	state := p.userWindows[key]
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
	kept = append(kept, tradeWindowEvent{eventID: event.ID, notional: event.Notional, eventTime: event.EventTime})
	state.events = kept
	state.total = total + event.Notional
	p.userWindows[key] = state
	return previousTotal
}

// userWindowTotal returns current in-process user window total after an update.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param event Market event identifying the window.
// @param userID User id for key isolation.
// @param ruleID Rule id for key isolation.
// @returns Current window total.
func (p Pipeline) userWindowTotal(event model.MarketEvent, userID int64, ruleID int64) float64 {
	if p.userWindowMu == nil || p.userWindows == nil {
		return 0
	}
	p.userWindowMu.Lock()
	defer p.userWindowMu.Unlock()
	return p.userWindows[userWindowKey{
		userID:     userID,
		ruleID:     ruleID,
		exchange:   event.Exchange,
		marketType: event.MarketType,
		symbol:     event.Symbol,
	}].total
}

// updateRedisUserWindow stores user window state in Redis sorted sets with expiry.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param event Market event entering the window.
// @param userID User id for key isolation.
// @param ruleID Rule id for key isolation.
// @param windowSec Window size in seconds.
// @returns Previous total, updated total, and Redis error.
func (p Pipeline) updateRedisUserWindow(ctx context.Context, event model.MarketEvent, userID int64, ruleID int64, windowSec int) (float64, float64, error) {
	key := fmt.Sprintf("window:user_rule:%d:%d:%s:%s:%s", userID, ruleID, event.Exchange, event.MarketType, event.Symbol)
	cutoff := event.EventTime.Add(-time.Duration(windowSec) * time.Second).UnixMilli()
	if err := p.redis.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(cutoff-1, 10)).Err(); err != nil {
		return 0, 0, err
	}
	items, err := p.redis.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return 0, 0, err
	}
	previousTotal := sumWindowMembers(items)
	member := fmt.Sprintf("%s:%f", event.ID, event.Notional)
	if err := p.redis.ZAdd(ctx, key, redis.Z{Score: float64(event.EventTime.UnixMilli()), Member: member}).Err(); err != nil {
		return 0, 0, err
	}
	if err := p.redis.Expire(ctx, key, time.Duration(windowSec+60)*time.Second).Err(); err != nil {
		return 0, 0, err
	}
	return previousTotal, previousTotal + event.Notional, nil
}

// sumWindowMembers sums notional values encoded in Redis window members.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param members Redis sorted-set member strings formatted as event_id:notional.
// @returns Total notional for parseable members.
func sumWindowMembers(members []string) float64 {
	total := 0.0
	for _, member := range members {
		for i := len(member) - 1; i >= 0; i-- {
			if member[i] != ':' {
				continue
			}
			value, err := strconv.ParseFloat(member[i+1:], 64)
			if err == nil {
				total += value
			}
			break
		}
	}
	return total
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

// allowUserAlert applies user-specific dedupe and rate-limit keys.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param alert Alert to dedupe.
// @param userID User id for key isolation.
// @returns Whether delivery is allowed and Redis error.
func (p Pipeline) allowUserAlert(ctx context.Context, alert model.Alert, userID int64) (bool, error) {
	if p.redis == nil {
		return true, nil
	}
	dedupeKey := fmt.Sprintf("dedupe:user_alert:%d:%s:%s", userID, alert.TriggerKey, alert.EventID)
	limitedKey := fmt.Sprintf("rate_limit:user_alert:%d:%s", userID, alert.TriggerKey)
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
