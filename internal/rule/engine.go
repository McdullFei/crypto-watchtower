package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

// userDigestItem stores one queued user alert for digest delivery.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type userDigestItem struct {
	UserID      int64       `json:"user_id"`
	ChatID      string      `json:"chat_id"`
	QueuedAt    time.Time   `json:"queued_at"`
	IntervalMin int         `json:"interval_min"`
	Alert       model.Alert `json:"alert"`
}

// userDigestState stores bounded digest items for one user in memory.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type userDigestState struct {
	items []userDigestItem
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
	userDigestMu    *sync.Mutex
	userDigests     map[int64]userDigestState
	userDigestMax   int
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
	p.userDigestMu = &sync.Mutex{}
	p.userDigests = make(map[int64]userDigestState)
	p.userDigestMax = 20
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
		} else if userQuietHoursActive(target.User, event.EventTime) {
			status = "quiet_hours"
			message = "telegram quiet hours"
		} else if target.User.TelegramDigestEnabled {
			status = "digested"
			message = "queued for telegram digest"
			if err := p.queueUserDigest(ctx, target.User, alert, event.EventTime); err != nil {
				return fmt.Errorf("queue user digest: %w", err)
			}
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

// FlushUserDigests sends due user digest summaries and clears flushed queues.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param now Current time used for due-window evaluation.
// @returns Error when digest storage or delivery fails.
func (p Pipeline) FlushUserDigests(ctx context.Context, now time.Time) error {
	if p.userSender == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if p.redis != nil {
		return p.flushRedisUserDigests(ctx, now)
	}
	return p.flushMemoryUserDigests(ctx, now)
}

// queueUserDigest appends one alert to a bounded per-user digest queue.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param user User that owns the digest queue.
// @param alert Alert to queue.
// @param queuedAt Queue timestamp.
// @returns Error when digest storage fails.
func (p Pipeline) queueUserDigest(ctx context.Context, user model.User, alert model.Alert, queuedAt time.Time) error {
	if queuedAt.IsZero() {
		queuedAt = time.Now().UTC()
	}
	item := userDigestItem{
		UserID:      user.ID,
		ChatID:      user.TelegramChatID,
		QueuedAt:    queuedAt.UTC(),
		IntervalMin: digestIntervalMin(user.TelegramDigestIntervalMin),
		Alert:       alert,
	}
	if p.redis != nil {
		return p.queueRedisUserDigest(ctx, item)
	}
	p.queueMemoryUserDigest(item)
	return nil
}

// queueMemoryUserDigest appends and trims one in-process user digest queue.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param item Digest item to queue.
func (p Pipeline) queueMemoryUserDigest(item userDigestItem) {
	if p.userDigestMu == nil || p.userDigests == nil {
		return
	}
	p.userDigestMu.Lock()
	defer p.userDigestMu.Unlock()

	state := p.userDigests[item.UserID]
	state.items = append(pruneDigestItems(state.items, item.QueuedAt), item)
	state.items = trimDigestItems(state.items, p.digestMaxItems())
	p.userDigests[item.UserID] = state
}

// queueRedisUserDigest appends and trims one Redis-backed user digest queue.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param item Digest item to queue.
// @returns Error when Redis persistence fails.
func (p Pipeline) queueRedisUserDigest(ctx context.Context, item userDigestItem) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	key := userDigestRedisKey(item.UserID)
	if err := p.redis.RPush(ctx, key, raw).Err(); err != nil {
		return err
	}
	if err := p.redis.LTrim(ctx, key, int64(-p.digestMaxItems()), -1).Err(); err != nil {
		return err
	}
	if err := p.pruneRedisUserDigest(ctx, key, item.QueuedAt); err != nil {
		return err
	}
	if err := p.redis.Expire(ctx, key, time.Duration(item.IntervalMin+60)*time.Minute).Err(); err != nil {
		return err
	}
	return p.redis.SAdd(ctx, "digest:user_ids", strconv.FormatInt(item.UserID, 10)).Err()
}

// pruneRedisUserDigest keeps a Redis digest queue inside the configured time and count bounds.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param key Redis list key.
// @param now Current queue timestamp used for time-window pruning.
// @returns Error when Redis access or item encoding fails.
func (p Pipeline) pruneRedisUserDigest(ctx context.Context, key string, now time.Time) error {
	rawItems, err := p.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return err
	}
	items := make([]userDigestItem, 0, len(rawItems))
	for _, raw := range rawItems {
		var item userDigestItem
		if err := json.Unmarshal([]byte(raw), &item); err == nil {
			items = append(items, item)
		}
	}
	items = trimDigestItems(pruneDigestItems(items, now), p.digestMaxItems())
	if err := p.redis.Del(ctx, key).Err(); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	encoded := make([]any, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			return err
		}
		encoded = append(encoded, raw)
	}
	return p.redis.RPush(ctx, key, encoded...).Err()
}

// flushMemoryUserDigests sends due in-process digest summaries.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param now Current time used for due-window evaluation.
// @returns Error when a digest send fails.
func (p Pipeline) flushMemoryUserDigests(ctx context.Context, now time.Time) error {
	if p.userDigestMu == nil || p.userDigests == nil {
		return nil
	}
	due := make([][]userDigestItem, 0)
	p.userDigestMu.Lock()
	for userID, state := range p.userDigests {
		if digestDue(state.items, now) {
			due = append(due, append([]userDigestItem(nil), state.items...))
			delete(p.userDigests, userID)
			continue
		}
		p.userDigests[userID] = state
	}
	p.userDigestMu.Unlock()
	for _, items := range due {
		if err := p.sendUserDigest(ctx, now, items); err != nil {
			return err
		}
	}
	return nil
}

// flushRedisUserDigests sends due Redis-backed digest summaries.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param now Current time used for due-window evaluation.
// @returns Error when Redis access or digest delivery fails.
func (p Pipeline) flushRedisUserDigests(ctx context.Context, now time.Time) error {
	userIDs, err := p.redis.SMembers(ctx, "digest:user_ids").Result()
	if err != nil {
		return err
	}
	for _, rawUserID := range userIDs {
		userID, err := strconv.ParseInt(rawUserID, 10, 64)
		if err != nil {
			_ = p.redis.SRem(ctx, "digest:user_ids", rawUserID).Err()
			continue
		}
		key := userDigestRedisKey(userID)
		rawItems, err := p.redis.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return err
		}
		items := make([]userDigestItem, 0, len(rawItems))
		for _, raw := range rawItems {
			var item userDigestItem
			if err := json.Unmarshal([]byte(raw), &item); err == nil {
				items = append(items, item)
			}
		}
		if !digestDue(items, now) {
			continue
		}
		if err := p.sendUserDigest(ctx, now, items); err != nil {
			return err
		}
		if err := p.redis.Del(ctx, key).Err(); err != nil {
			return err
		}
		if err := p.redis.SRem(ctx, "digest:user_ids", rawUserID).Err(); err != nil {
			return err
		}
	}
	return nil
}

// sendUserDigest sends one summarized digest alert.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param ctx Request context.
// @param now Current time used for summary metadata.
// @param items Digest items to summarize.
// @returns Error when the user sender fails.
func (p Pipeline) sendUserDigest(ctx context.Context, now time.Time, items []userDigestItem) error {
	if len(items) == 0 {
		return nil
	}
	first := items[0]
	return p.userSender.SendTo(ctx, first.ChatID, model.Alert{
		ID:          fmt.Sprintf("user-digest-%d-%d", first.UserID, now.Unix()),
		RuleID:      "user_digest",
		Exchange:    "user",
		MarketType:  "digest",
		Symbol:      "DIGEST",
		Type:        "user_digest",
		Severity:    "info",
		Title:       fmt.Sprintf("Telegram digest: %d alerts", len(items)),
		Message:     digestMessage(items),
		TriggerKey:  fmt.Sprintf("user:%d:digest", first.UserID),
		TriggerTime: now.UTC(),
		CreatedAt:   now.UTC(),
	})
}

// digestMessage builds a bounded human-readable digest body.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param items Digest items to summarize.
// @returns Digest body text.
func digestMessage(items []userDigestItem) string {
	parts := make([]string, 0, len(items)+1)
	parts = append(parts, fmt.Sprintf("%d queued user alerts:", len(items)))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("- %s %s %s", item.Alert.Symbol, item.Alert.Type, item.Alert.Title))
	}
	return strings.Join(parts, "\n")
}

// digestDue reports whether queued items are ready to flush.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param items Digest items for one user.
// @param now Current time used for due-window evaluation.
// @returns Whether the digest queue should be flushed.
func digestDue(items []userDigestItem, now time.Time) bool {
	if len(items) == 0 {
		return false
	}
	interval := time.Duration(digestIntervalMin(items[0].IntervalMin)) * time.Minute
	return !now.Before(items[0].QueuedAt.Add(interval))
}

// pruneDigestItems drops items outside the oldest queued item's configured window.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param items Digest items to prune.
// @param now Current time used for time-window pruning.
// @returns Items retained inside the digest window.
func pruneDigestItems(items []userDigestItem, now time.Time) []userDigestItem {
	if len(items) == 0 {
		return items
	}
	interval := time.Duration(digestIntervalMin(items[len(items)-1].IntervalMin)) * time.Minute
	cutoff := now.Add(-interval)
	kept := items[:0]
	for _, item := range items {
		if item.QueuedAt.Before(cutoff) {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

// trimDigestItems bounds a digest queue by maximum item count.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param items Digest items to trim.
// @param maxItems Maximum retained items.
// @returns Trimmed digest items.
func trimDigestItems(items []userDigestItem, maxItems int) []userDigestItem {
	if maxItems <= 0 || len(items) <= maxItems {
		return items
	}
	return items[len(items)-maxItems:]
}

// digestMaxItems returns the configured per-user digest item cap.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @returns Maximum retained digest items per user.
func (p Pipeline) digestMaxItems() int {
	if p.userDigestMax <= 0 {
		return 20
	}
	return p.userDigestMax
}

// digestIntervalMin normalizes digest interval minutes.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param value Requested interval minutes.
// @returns Bounded interval minutes.
func digestIntervalMin(value int) int {
	if value < 5 {
		return 60
	}
	if value > 1440 {
		return 1440
	}
	return value
}

// userDigestRedisKey returns a Redis key for one user's digest queue.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param userID User id that owns the digest queue.
// @returns Redis list key.
func userDigestRedisKey(userID int64) string {
	return fmt.Sprintf("digest:user:%d", userID)
}

// userQuietHoursActive reports whether a user's quiet-hours window contains the event time.
//
// Author: __AUTHOR__
// Date: 2026-07-01
// @param user User containing quiet-hours preferences.
// @param eventTime Event time to evaluate.
// @returns Whether Telegram delivery should be suppressed.
func userQuietHoursActive(user model.User, eventTime time.Time) bool {
	if !user.TelegramQuietHoursEnabled {
		return false
	}
	location, err := time.LoadLocation(user.TelegramQuietHoursTimezone)
	if err != nil {
		return false
	}
	start, err := time.Parse("15:04", user.TelegramQuietHoursStart)
	if err != nil {
		return false
	}
	end, err := time.Parse("15:04", user.TelegramQuietHoursEnd)
	if err != nil {
		return false
	}
	local := eventTime.In(location)
	currentMinute := local.Hour()*60 + local.Minute()
	startMinute := start.Hour()*60 + start.Minute()
	endMinute := end.Hour()*60 + end.Minute()
	if startMinute == endMinute {
		return true
	}
	if startMinute < endMinute {
		return currentMinute >= startMinute && currentMinute < endMinute
	}
	return currentMinute >= startMinute || currentMinute < endMinute
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
