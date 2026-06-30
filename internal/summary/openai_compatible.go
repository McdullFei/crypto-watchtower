package summary

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatibleConfig contains chat-completions provider settings.
//
// Author: monsterfei
// Date: 2026-06-30
type OpenAICompatibleConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	TimeoutSec int
	Disclaimer string
}

// OpenAICompatibleGenerator calls an OpenAI-compatible chat-completions endpoint.
//
// Author: monsterfei
// Date: 2026-06-30
type OpenAICompatibleGenerator struct {
	cfg    OpenAICompatibleConfig
	client *http.Client
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// NewOpenAICompatibleGenerator creates a chat-completions backed generator.
//
// Author: monsterfei
// Date: 2026-06-30
// @param cfg Provider configuration.
// @param client Optional HTTP client.
// @returns OpenAI-compatible generator instance.
func NewOpenAICompatibleGenerator(cfg OpenAICompatibleConfig, client *http.Client) OpenAICompatibleGenerator {
	if cfg.Disclaimer == "" {
		cfg.Disclaimer = defaultDisclaimer
	}
	if client == nil {
		client = &http.Client{}
	}
	if cfg.TimeoutSec > 0 && client.Timeout == 0 {
		copied := *client
		copied.Timeout = time.Duration(cfg.TimeoutSec) * time.Second
		client = &copied
	}
	return OpenAICompatibleGenerator{cfg: cfg, client: client}
}

// Generate creates one summary by calling the configured provider.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param snapshot Bounded market snapshot.
// @returns Provider summary text containing the configured disclaimer.
func (g OpenAICompatibleGenerator) Generate(ctx context.Context, snapshot Snapshot) (string, error) {
	requestBody := chatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "你是币圈异动监控平台的市场摘要助手，只基于输入数据总结风险和异动，不给交易建议。",
			},
			{
				Role:    "user",
				Content: buildPrompt(snapshot, g.cfg.Disclaimer),
			},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(g.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("openai compatible summary request failed: status %d", resp.StatusCode)
	}

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		return "", errors.New("openai compatible summary response is empty")
	}
	return ensureDisclaimer(decoded.Choices[0].Message.Content, g.cfg.Disclaimer), nil
}

// buildPrompt renders the bounded snapshot for the provider request.
//
// Author: monsterfei
// Date: 2026-06-30
// @param snapshot Bounded market snapshot.
// @param disclaimer Required disclaimer text.
// @returns Prompt text for the provider.
func buildPrompt(snapshot Snapshot, disclaimer string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("window_from=%s\n", snapshot.WindowFrom.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("window_to=%s\n", snapshot.WindowTo.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("alert_count=%d\n", snapshot.AlertCount))
	builder.WriteString(fmt.Sprintf("event_count=%d\n", snapshot.EventCount))
	builder.WriteString(fmt.Sprintf("funding_count=%d\n", snapshot.FundingCount))
	builder.WriteString("alerts:\n")
	for _, alert := range snapshot.Alerts {
		builder.WriteString(fmt.Sprintf("- %s %s %s %s\n", alert.Exchange, alert.Symbol, alert.Type, alert.Title))
	}
	builder.WriteString("events:\n")
	for _, event := range snapshot.Events {
		builder.WriteString(fmt.Sprintf("- %s %s %s notional=%.2f\n", event.Exchange, event.Symbol, event.EventType, event.Notional))
	}
	builder.WriteString("必须包含免责声明：")
	builder.WriteString(disclaimer)
	return builder.String()
}

// ensureDisclaimer appends disclaimer text when provider output omits it.
//
// Author: monsterfei
// Date: 2026-06-30
// @param content Provider response content.
// @param disclaimer Required disclaimer text.
// @returns Content guaranteed to include the disclaimer.
func ensureDisclaimer(content string, disclaimer string) string {
	if disclaimer == "" {
		disclaimer = defaultDisclaimer
	}
	if strings.Contains(content, disclaimer) {
		return content
	}
	return strings.TrimSpace(content) + "\n\n" + disclaimer
}
