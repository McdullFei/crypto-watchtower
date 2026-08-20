package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// memoryTelegramBindingTokens stores binding tokens for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
type memoryTelegramBindingTokens struct {
	tokens map[string]model.TelegramBindingToken
}

// memoryTelegramAccountBinder records account chat bindings for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
type memoryTelegramAccountBinder struct {
	bindings map[int64]string
}

// newMemoryTelegramBindingTokens creates an empty binding-token repository for tests.
//
// Author: monsterfei
// Date: 2026-07-01
func newMemoryTelegramBindingTokens() *memoryTelegramBindingTokens {
	return &memoryTelegramBindingTokens{tokens: map[string]model.TelegramBindingToken{}}
}

// newMemoryTelegramAccountBinder creates an empty account binder for tests.
//
// Author: monsterfei
// Date: 2026-07-01
func newMemoryTelegramAccountBinder() *memoryTelegramAccountBinder {
	return &memoryTelegramAccountBinder{bindings: map[int64]string{}}
}

// Create stores one Telegram binding token for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (m *memoryTelegramBindingTokens) Create(_ context.Context, token model.TelegramBindingToken) error {
	if _, ok := m.tokens[token.TokenHash]; ok {
		return errors.New("duplicate token")
	}
	m.tokens[token.TokenHash] = token
	return nil
}

// FindActiveByHash returns one active Telegram binding token for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (m *memoryTelegramBindingTokens) FindActiveByHash(_ context.Context, tokenHash string, now time.Time) (model.TelegramBindingToken, bool, error) {
	token, ok := m.tokens[tokenHash]
	if !ok || token.UsedAt != nil || !token.ExpiresAt.After(now) {
		return model.TelegramBindingToken{}, false, nil
	}
	return token, true, nil
}

// MarkUsed consumes one Telegram binding token for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (m *memoryTelegramBindingTokens) MarkUsed(_ context.Context, tokenHash string, now time.Time) error {
	token, ok := m.tokens[tokenHash]
	if !ok {
		return nil
	}
	token.UsedAt = &now
	m.tokens[tokenHash] = token
	return nil
}

// BindTelegramChat records one Telegram chat id for service tests.
//
// Author: monsterfei
// Date: 2026-07-01
func (m *memoryTelegramAccountBinder) BindTelegramChat(_ context.Context, userID int64, chatID string) error {
	m.bindings[userID] = chatID
	return nil
}

// TestTelegramBindingServiceCreatesHashedToken verifies raw binding tokens are not persisted.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramBindingServiceCreatesHashedToken(t *testing.T) {
	tokens := newMemoryTelegramBindingTokens()
	service := NewTelegramBindingService(tokens, newMemoryTelegramAccountBinder(), TelegramBindingConfig{TokenTTL: time.Hour})

	rawToken, expiresAt, err := service.CreateBindingToken(context.Background(), 42)
	if err != nil {
		t.Fatalf("create binding token: %v", err)
	}
	if rawToken == "" {
		t.Fatal("expected raw binding token")
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected future expiry, got %s", expiresAt)
	}
	if len(tokens.tokens) != 1 {
		t.Fatalf("expected one stored token, got %d", len(tokens.tokens))
	}
	for tokenHash, stored := range tokens.tokens {
		if tokenHash == rawToken || stored.TokenHash == rawToken {
			t.Fatalf("raw binding token must not be persisted: %q", rawToken)
		}
		if len(tokenHash) != 64 || stored.UserID != 42 {
			t.Fatalf("unexpected stored token: hash=%q token=%+v", tokenHash, stored)
		}
	}
}

// TestTelegramBindingServiceConsumesTokenOnce verifies binding tokens bind one chat and cannot be reused.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramBindingServiceConsumesTokenOnce(t *testing.T) {
	tokens := newMemoryTelegramBindingTokens()
	accounts := newMemoryTelegramAccountBinder()
	service := NewTelegramBindingService(tokens, accounts, TelegramBindingConfig{TokenTTL: time.Hour})
	rawToken, _, err := service.CreateBindingToken(context.Background(), 42)
	if err != nil {
		t.Fatalf("create binding token: %v", err)
	}

	userID, ok, err := service.BindTelegramChat(context.Background(), rawToken, "12345")
	if err != nil || !ok || userID != 42 {
		t.Fatalf("bind telegram chat: user_id=%d ok=%v err=%v", userID, ok, err)
	}
	if accounts.bindings[42] != "12345" {
		t.Fatalf("expected account binding, got %+v", accounts.bindings)
	}
	if _, ok, err := service.BindTelegramChat(context.Background(), rawToken, "67890"); err != nil || ok {
		t.Fatalf("expected token reuse to fail, ok=%v err=%v", ok, err)
	}
}
