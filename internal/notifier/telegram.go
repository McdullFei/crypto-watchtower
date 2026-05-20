package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type TelegramNotifier struct {
	BotToken string
	ChatID   string
	Client   *http.Client
}

func NewTelegramNotifier(botToken, chatID string, client *http.Client) TelegramNotifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
		Client:   client,
	}
}

func (n TelegramNotifier) Send(ctx context.Context, alert model.Alert) error {
	if n.BotToken == "" || n.ChatID == "" {
		return errors.New("telegram notifier is not configured")
	}
	body, err := json.Marshal(map[string]string{
		"chat_id": n.ChatID,
		"text":    FormatAlert(alert),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.telegram.org/bot"+n.BotToken+"/sendMessage",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return errors.New("telegram send failed")
	}
	return nil
}
