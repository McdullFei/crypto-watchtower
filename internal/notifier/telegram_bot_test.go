package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type stubRuleLister struct {
	rules []model.AlertRule
}

func (s stubRuleLister) ListEnabled(context.Context) ([]model.AlertRule, error) {
	return s.rules, nil
}

type stubUserBinder struct {
	mu      sync.Mutex
	chatIDs []string
}

// stubTelegramTokenBinder records token-based Telegram bindings for poller tests.
//
// Author: monsterfei
// Date: 2026-07-01
type stubTelegramTokenBinder struct {
	token  string
	chatID string
}

// BindTelegramChat records one token binding request for poller tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (s *stubTelegramTokenBinder) BindTelegramChat(_ context.Context, token string, chatID string) (int64, bool, error) {
	s.token = token
	s.chatID = chatID
	return 42, true, nil
}

func (s *stubUserBinder) UpsertTelegramChat(_ context.Context, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatIDs = append(s.chatIDs, chatID)
	return nil
}

func TestTelegramPollerHandlesStartAndRulesCommands(t *testing.T) {
	var sentMessages []string
	var sentChatIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot/token/getUpdates":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": []map[string]any{
					{
						"update_id": 101,
						"message": map[string]any{
							"message_id": 1,
							"chat": map[string]any{
								"id": 12345,
							},
							"text": "/start",
						},
					},
					{
						"update_id": 102,
						"message": map[string]any{
							"message_id": 2,
							"chat": map[string]any{
								"id": 12345,
							},
							"text": "/status",
						},
					},
					{
						"update_id": 103,
						"message": map[string]any{
							"message_id": 3,
							"chat": map[string]any{
								"id": 12345,
							},
							"text": "/rules",
						},
					},
				},
			})
		case "/bot/token/sendMessage":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode sendMessage payload: %v", err)
			}
			sentChatIDs = append(sentChatIDs, payload["chat_id"].(string))
			sentMessages = append(sentMessages, payload["text"].(string))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	binder := &stubUserBinder{}
	client := NewTelegramClient("token", server.URL+"/bot", server.Client())
	poller := NewTelegramPoller(client, binder, stubRuleLister{
		rules: []model.AlertRule{
			{Symbol: "BTCUSDT", RuleType: "large_trade", Threshold: 100000, Enabled: true},
		},
	}, nil, TelegramPollerConfig{})

	if err := poller.PollOnce(context.Background(), 0); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if len(binder.chatIDs) != 1 || binder.chatIDs[0] != "12345" {
		t.Fatalf("expected chat binding, got %+v", binder.chatIDs)
	}
	if len(sentMessages) != 3 {
		t.Fatalf("expected 3 replies, got %d", len(sentMessages))
	}
	if sentChatIDs[0] != "12345" || sentChatIDs[1] != "12345" || sentChatIDs[2] != "12345" {
		t.Fatalf("expected replies to same chat, got %+v", sentChatIDs)
	}
	if !strings.Contains(sentMessages[0], "default alert channel") {
		t.Fatalf("expected start reply to explain default alert channel behavior, got %q", sentMessages[0])
	}
}

// TestTelegramPollerHandlesStartBindingToken verifies /start token binds a chat to an account.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramPollerHandlesStartBindingToken(t *testing.T) {
	var sentMessage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot/token/getUpdates":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": []map[string]any{
					{
						"update_id": 301,
						"message": map[string]any{
							"message_id": 1,
							"chat": map[string]any{
								"id": 12345,
							},
							"text": "/start bind-token",
						},
					},
				},
			})
		case "/bot/token/sendMessage":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode sendMessage payload: %v", err)
			}
			sentMessage = payload["text"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	binding := &stubTelegramTokenBinder{}
	client := NewTelegramClient("token", server.URL+"/bot", server.Client())
	poller := NewTelegramPoller(client, &stubUserBinder{}, stubRuleLister{}, binding, TelegramPollerConfig{})

	if err := poller.PollOnce(context.Background(), 0); err != nil {
		t.Fatalf("poll once: %v", err)
	}
	if binding.token != "bind-token" || binding.chatID != "12345" {
		t.Fatalf("unexpected binding call: %+v", binding)
	}
	if !strings.Contains(sentMessage, "bound") {
		t.Fatalf("expected bound response, got %q", sentMessage)
	}
}

func TestTelegramPollerHandlesTestCommand(t *testing.T) {
	var sentMessage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot/token/getUpdates":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": []map[string]any{
					{
						"update_id": 201,
						"message": map[string]any{
							"message_id": 1,
							"chat": map[string]any{
								"id": 54321,
							},
							"text": "/test",
						},
					},
				},
			})
		case "/bot/token/sendMessage":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode sendMessage payload: %v", err)
			}
			sentMessage = payload["text"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewTelegramClient("token", server.URL+"/bot", server.Client())
	poller := NewTelegramPoller(client, &stubUserBinder{}, stubRuleLister{}, nil, TelegramPollerConfig{
		TestAlert: model.Alert{
			Symbol:  "BTCUSDT",
			Title:   "Telegram test",
			Message: "CryptoWatchtower test alert",
		},
	})

	if err := poller.PollOnce(context.Background(), 0); err != nil {
		t.Fatalf("poll once: %v", err)
	}
	if sentMessage == "" {
		t.Fatal("expected test message to be sent")
	}
}

// TestTelegramNotifierLocalSandboxSucceeds verifies SIT can exercise sent notification logs without real Telegram credentials.
//
// Author: monsterfei
// Date: 2026-07-02
func TestTelegramNotifierLocalSandboxSucceeds(t *testing.T) {
	notifier := NewTelegramNotifier("local-sandbox", "sit-chat", nil)

	err := notifier.Send(context.Background(), model.Alert{
		Symbol:  "BTCUSDT",
		Type:    "large_trade",
		Title:   "Large trade detected",
		Message: "exchange=binance symbol=BTCUSDT threshold=1000 notional=2000",
	})

	if err != nil {
		t.Fatalf("expected local sandbox send to succeed, got %v", err)
	}
}

func TestTelegramNotifierRetriesTemporaryFailures(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	notifier := NewTelegramNotifier("token", "12345", server.Client())
	notifier.BaseURL = server.URL
	notifier.RetryMaxAttempts = 3
	notifier.RetryBackoff = 0

	err := notifier.Send(context.Background(), model.Alert{
		Title:   "retry me",
		Message: "temporary failure path",
	})
	if err != nil {
		t.Fatalf("send with retry: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// TestTelegramClientRedactsBotTokenFromNetworkErrors verifies request URLs cannot leak credentials through errors.
//
// Author: monsterfei
// Date: 2026-09-02
// @param t Testing context.
func TestTelegramClientRedactsBotTokenFromNetworkErrors(t *testing.T) {
	const botToken = "sit-secret-bot-token"
	client := NewTelegramClient(botToken, "http://telegram.invalid/bot", &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network unavailable")
		}),
	})

	err := client.SendMessage(context.Background(), "100001", "test")
	if err == nil {
		t.Fatal("expected Telegram network error")
	}
	if strings.Contains(err.Error(), botToken) || len(err.Error()) > 128 {
		t.Fatalf("expected bounded redacted Telegram error, got %q", err.Error())
	}
}

// roundTripFunc adapts a function into an HTTP transport for notifier tests.
//
// Author: monsterfei
// Date: 2026-09-02
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes the configured notifier test transport.
//
// Author: monsterfei
// Date: 2026-09-02
// @param request Outgoing HTTP request.
// @returns HTTP response and transport error.
func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
