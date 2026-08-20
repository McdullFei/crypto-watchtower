package notifier

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type TelegramNotifier struct {
	BotToken         string
	ChatID           string
	Client           *http.Client
	BaseURL          string
	RetryMaxAttempts int
	RetryBackoff     time.Duration
}

func NewTelegramNotifier(botToken, chatID string, client *http.Client) TelegramNotifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
		Client:   client,
		BaseURL:  "https://api.telegram.org/bot",
	}
}

func (n TelegramNotifier) Send(ctx context.Context, alert model.Alert) error {
	return n.SendTo(ctx, n.ChatID, alert)
}

// SendTo sends one alert to an explicit Telegram chat id.
//
// Author: monsterfei
// Date: 2026-07-01
// modified by monsterfei on 2026-07-02
// @param ctx Request context.
// @param chatID Telegram chat id.
// @param alert Alert to format and send.
// @returns Error when Telegram delivery fails.
func (n TelegramNotifier) SendTo(ctx context.Context, chatID string, alert model.Alert) error {
	if n.BotToken == "" || chatID == "" {
		return errors.New("telegram notifier is not configured")
	}
	if n.BotToken == "local-sandbox" {
		return nil
	}

	attempts := n.RetryMaxAttempts
	if attempts <= 0 {
		attempts = 3
	}
	backoff := n.RetryBackoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	client := NewTelegramClient(n.BotToken, n.BaseURL, n.Client)

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = client.SendMessage(ctx, chatID, FormatAlert(alert))
		if lastErr == nil {
			return nil
		}
		if !isRetryableTelegramError(lastErr) || attempt == attempts {
			return lastErr
		}
		if err := sleepContext(ctx, backoff); err != nil {
			return lastErr
		}
		backoff *= 2
	}
	return lastErr
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
