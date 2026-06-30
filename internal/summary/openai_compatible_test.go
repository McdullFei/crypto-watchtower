package summary

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestOpenAICompatibleGeneratorSendsSnapshotPrompt verifies prompt and auth request shape.
//
// Author: monsterfei
// Date: 2026-06-30
func TestOpenAICompatibleGeneratorSendsSnapshotPrompt(t *testing.T) {
	var requestPath string
	var authorization string
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		var body chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		for _, message := range body.Messages {
			if message.Role == "user" {
				prompt = message.Content
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"摘要。不构成投资建议"}}]}`))
	}))
	defer server.Close()

	generator := NewOpenAICompatibleGenerator(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		APIKey:     "secret",
		Model:      "summary-model",
		TimeoutSec: 3,
		Disclaimer: "不构成投资建议",
	}, server.Client())

	_, err := generator.Generate(context.Background(), Snapshot{
		WindowFrom:   time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
		WindowTo:     time.Date(2026, 6, 29, 12, 15, 0, 0, time.UTC),
		AlertCount:   2,
		EventCount:   3,
		FundingCount: 1,
	})
	if err != nil {
		t.Fatalf("generate openai compatible summary: %v", err)
	}
	if requestPath != "/chat/completions" {
		t.Fatalf("unexpected path: %s", requestPath)
	}
	if authorization != "Bearer secret" {
		t.Fatalf("unexpected authorization: %s", authorization)
	}
	for _, want := range []string{"alert_count=2", "event_count=3", "funding_count=1", "不构成投资建议"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got %s", want, prompt)
		}
	}
}

// TestOpenAICompatibleGeneratorRequiresDisclaimerInResponse verifies risk text is never omitted.
//
// Author: monsterfei
// Date: 2026-06-30
func TestOpenAICompatibleGeneratorRequiresDisclaimerInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"摘要内容"}}]}`))
	}))
	defer server.Close()

	generator := NewOpenAICompatibleGenerator(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		APIKey:     "secret",
		Model:      "summary-model",
		TimeoutSec: 3,
		Disclaimer: "不构成投资建议",
	}, server.Client())

	content, err := generator.Generate(context.Background(), Snapshot{})
	if err != nil {
		t.Fatalf("generate openai compatible summary: %v", err)
	}
	if !strings.Contains(content, "不构成投资建议") {
		t.Fatalf("expected appended disclaimer, got %s", content)
	}
}
