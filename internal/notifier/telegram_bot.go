package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type TelegramClient struct {
	BotToken string
	BaseURL  string
	Client   *http.Client
}

func NewTelegramClient(botToken, baseURL string, client *http.Client) TelegramClient {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://api.telegram.org/bot"
	}
	return TelegramClient{
		BotToken: botToken,
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Client:   client,
	}
}

type TelegramPollerConfig struct {
	PollTimeoutSec int
	TestAlert      model.Alert
	StatusText     string
}

type TelegramUserBinder interface {
	UpsertTelegramChat(context.Context, string) error
}

type TelegramRuleLister interface {
	ListEnabled(context.Context) ([]model.AlertRule, error)
}

type TelegramPoller struct {
	client TelegramClient
	users  TelegramUserBinder
	rules  TelegramRuleLister
	config TelegramPollerConfig
	offset int64
}

func NewTelegramPoller(
	client TelegramClient,
	users TelegramUserBinder,
	rules TelegramRuleLister,
	config TelegramPollerConfig,
) *TelegramPoller {
	if config.PollTimeoutSec <= 0 {
		config.PollTimeoutSec = 10
	}
	if config.StatusText == "" {
		config.StatusText = "CryptoWatchtower is running."
	}
	if config.TestAlert.Title == "" {
		config.TestAlert = model.Alert{
			Symbol:  "BTCUSDT",
			Title:   "Telegram test",
			Message: "CryptoWatchtower test alert",
		}
	}
	return &TelegramPoller{
		client: client,
		users:  users,
		rules:  rules,
		config: config,
	}
}

func (p *TelegramPoller) Start(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := p.PollOnce(ctx, p.offset); err != nil {
			if err := sleepContext(ctx, time.Second); err != nil {
				return ctx.Err()
			}
		}
	}
}

func (p *TelegramPoller) PollOnce(ctx context.Context, offset int64) error {
	updates, err := p.client.GetUpdates(ctx, offset, p.config.PollTimeoutSec)
	if err != nil {
		return err
	}
	for _, update := range updates {
		if update.UpdateID >= p.offset {
			p.offset = update.UpdateID + 1
		}
		if err := p.handleMessage(ctx, update.Message); err != nil {
			return err
		}
	}
	return nil
}

func (p *TelegramPoller) handleMessage(ctx context.Context, message telegramMessage) error {
	command := normalizeTelegramCommand(message.Text)
	if command == "" {
		return nil
	}

	chatID := message.Chat.ChatIDString()
	if chatID == "" {
		return nil
	}

	switch command {
	case "/start":
		if p.users != nil {
			if err := p.users.UpsertTelegramChat(ctx, chatID); err != nil {
				return err
			}
		}
		return p.client.SendMessage(ctx, chatID, "CryptoWatchtower Telegram binding completed.")
	case "/status":
		return p.client.SendMessage(ctx, chatID, p.config.StatusText)
	case "/rules":
		text := "No enabled rules."
		if p.rules != nil {
			rules, err := p.rules.ListEnabled(ctx)
			if err != nil {
				return err
			}
			text = formatRuleList(rules)
		}
		return p.client.SendMessage(ctx, chatID, text)
	case "/test":
		return p.client.SendMessage(ctx, chatID, FormatAlert(p.config.TestAlert))
	default:
		return p.client.SendMessage(ctx, chatID, "Available commands: /start /status /rules /test")
	}
}

func normalizeTelegramCommand(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || text[0] != '/' {
		return ""
	}
	fields := strings.Fields(text)
	command := fields[0]
	if idx := strings.Index(command, "@"); idx >= 0 {
		command = command[:idx]
	}
	return command
}

func formatRuleList(rules []model.AlertRule) string {
	if len(rules) == 0 {
		return "No enabled rules."
	}
	lines := make([]string, 0, len(rules)+1)
	lines = append(lines, "Enabled rules:")
	for _, rule := range rules {
		lines = append(lines, fmt.Sprintf("%s %s %.2f", rule.Symbol, rule.RuleType, rule.Threshold))
	}
	return strings.Join(lines, "\n")
}

type telegramUpdateResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID int64           `json:"update_id"`
	Message  telegramMessage `json:"message"`
}

type telegramMessage struct {
	Text string       `json:"text"`
	Chat telegramChat `json:"chat"`
}

type telegramChat struct {
	ID json.Number `json:"id"`
}

func (c telegramChat) ChatIDString() string {
	return c.ID.String()
}

func (c TelegramClient) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]telegramUpdate, error) {
	payload := map[string]any{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message"},
	}
	req, err := c.newJSONRequest(ctx, "getUpdates", payload)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, telegramHTTPError{StatusCode: resp.StatusCode}
	}

	var out telegramUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, errors.New("telegram getUpdates returned not ok")
	}
	return out.Result, nil
}

func (c TelegramClient) SendMessage(ctx context.Context, chatID, text string) error {
	req, err := c.newJSONRequest(ctx, "sendMessage", map[string]string{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return telegramHTTPError{StatusCode: resp.StatusCode}
	}
	return nil
}

func (c TelegramClient) newJSONRequest(ctx context.Context, method string, body any) (*http.Request, error) {
	if c.BotToken == "" {
		return nil, errors.New("telegram bot token is required")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpointURL(method),
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c TelegramClient) endpointURL(method string) string {
	base := c.BaseURL
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + c.BotToken + "/" + method
}

type telegramHTTPError struct {
	StatusCode int
}

func (e telegramHTTPError) Error() string {
	return "telegram request failed: " + strconv.Itoa(e.StatusCode)
}

func isRetryableTelegramError(err error) bool {
	var httpErr telegramHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= http.StatusInternalServerError
	}
	return true
}
