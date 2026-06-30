package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// WebhookNotifier sends formatted alerts to a Discord-compatible JSON webhook.
//
// Author: monsterfei
// Date: 2026-06-29
type WebhookNotifier struct {
	URL     string
	Channel string
	Client  *http.Client
}

// NewWebhookNotifier creates a JSON webhook sender.
//
// Author: monsterfei
// Date: 2026-06-29
// @param url Webhook endpoint URL.
// @param channel Notification channel name used by callers.
// @param client Optional HTTP client.
// @returns Configured webhook notifier.
func NewWebhookNotifier(url string, channel string, client *http.Client) WebhookNotifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if channel == "" {
		channel = "webhook"
	}
	return WebhookNotifier{
		URL:     url,
		Channel: channel,
		Client:  client,
	}
}

// Send posts one formatted alert to the webhook endpoint.
//
// Author: monsterfei
// Date: 2026-06-29
// @param ctx Request context.
// @param alert Alert payload to format and send.
// @returns Error when configuration, request creation, transport, or response status fails.
func (n WebhookNotifier) Send(ctx context.Context, alert model.Alert) error {
	if n.URL == "" {
		return errors.New("webhook notifier is not configured")
	}
	payload := struct {
		Content string `json:"content"`
	}{
		Content: FormatAlert(alert),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
