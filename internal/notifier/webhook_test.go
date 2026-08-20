package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// TestWebhookNotifierSendsFormattedAlert verifies webhook payloads contain formatted alerts.
//
// Author: monsterfei
// Date: 2026-06-29
func TestWebhookNotifierSendsFormattedAlert(t *testing.T) {
	var payload struct {
		Content string `json:"content"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("expected json content type, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL, "discord", server.Client())
	err := notifier.Send(context.Background(), model.Alert{
		Title:   "BTCUSDT large aggressive flow",
		Message: "成交额: 150000 USDT",
	})
	if err != nil {
		t.Fatalf("send webhook: %v", err)
	}
	if !strings.Contains(payload.Content, "BTCUSDT large aggressive flow") || !strings.Contains(payload.Content, "150000") {
		t.Fatalf("expected formatted alert content, got %q", payload.Content)
	}
}

// TestWebhookNotifierReturnsErrorOnNon2xx verifies failed webhook responses are surfaced.
//
// Author: monsterfei
// Date: 2026-06-29
func TestWebhookNotifierReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL, "discord", server.Client())
	err := notifier.Send(context.Background(), model.Alert{Title: "test"})
	if err == nil {
		t.Fatal("expected non-2xx webhook response to return error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}

// TestWebhookNotifierRedactsTransportErrorURL verifies transport errors do not leak webhook secrets.
//
// Author: monsterfei
// Date: 2026-07-02
func TestWebhookNotifierRedactsTransportErrorURL(t *testing.T) {
	notifier := NewWebhookNotifier("http://127.0.0.1:1/webhook?secret=secret-token", "discord", &http.Client{})

	err := notifier.Send(context.Background(), model.Alert{Title: "test"})

	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "127.0.0.1:1/webhook") {
		t.Fatalf("expected redacted transport error, got %v", err)
	}
}
