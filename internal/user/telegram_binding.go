package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

const defaultTelegramBindingTTL = 10 * time.Minute

// TelegramBindingConfig controls Telegram binding token behavior.
//
// Author: monsterfei
// Date: 2026-07-01
type TelegramBindingConfig struct {
	TokenTTL time.Duration
}

// TelegramBindingTokenRepository defines persisted binding token operations.
//
// Author: monsterfei
// Date: 2026-07-01
type TelegramBindingTokenRepository interface {
	Create(context.Context, model.TelegramBindingToken) error
	FindActiveByHash(context.Context, string, time.Time) (model.TelegramBindingToken, bool, error)
	MarkUsed(context.Context, string, time.Time) error
}

// TelegramAccountBinder defines account chat-id binding operations.
//
// Author: monsterfei
// Date: 2026-07-01
type TelegramAccountBinder interface {
	BindTelegramChat(context.Context, int64, string) error
}

// TelegramBindingService coordinates account Telegram binding tokens.
//
// Author: monsterfei
// Date: 2026-07-01
type TelegramBindingService struct {
	tokens   TelegramBindingTokenRepository
	accounts TelegramAccountBinder
	cfg      TelegramBindingConfig
}

// NewTelegramBindingService creates a Telegram binding service.
//
// Author: monsterfei
// Date: 2026-07-01
// @param tokens Binding token repository.
// @param accounts Account chat-id binder.
// @param cfg Binding token configuration.
// @returns Telegram binding service.
func NewTelegramBindingService(tokens TelegramBindingTokenRepository, accounts TelegramAccountBinder, cfg TelegramBindingConfig) TelegramBindingService {
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = defaultTelegramBindingTTL
	}
	return TelegramBindingService{tokens: tokens, accounts: accounts, cfg: cfg}
}

// CreateBindingToken creates one raw Telegram binding token and stores only its hash.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param userID User id that owns the token.
// @returns Raw binding token, expiry timestamp, and persistence error.
func (s TelegramBindingService) CreateBindingToken(ctx context.Context, userID int64) (string, time.Time, error) {
	if s.tokens == nil {
		return "", time.Time{}, errors.New("telegram binding token repository is not configured")
	}
	if userID <= 0 {
		return "", time.Time{}, errors.New("user_id must be greater than 0")
	}
	rawToken, err := randomBindingToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.TokenTTL)
	if err := s.tokens.Create(ctx, model.TelegramBindingToken{
		UserID:    userID,
		TokenHash: hashBindingToken(rawToken),
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		return "", time.Time{}, err
	}
	return rawToken, expiresAt, nil
}

// BindTelegramChat consumes one binding token and stores the Telegram chat id.
//
// Author: monsterfei
// Date: 2026-07-01
// @param ctx Request context.
// @param rawToken Raw binding token from Telegram command.
// @param chatID Telegram chat id to bind.
// @returns Bound user id, whether the token was valid, and persistence error.
func (s TelegramBindingService) BindTelegramChat(ctx context.Context, rawToken string, chatID string) (int64, bool, error) {
	if s.tokens == nil || s.accounts == nil {
		return 0, false, errors.New("telegram binding repositories are not configured")
	}
	rawToken = strings.TrimSpace(rawToken)
	chatID = strings.TrimSpace(chatID)
	if rawToken == "" || chatID == "" {
		return 0, false, nil
	}
	now := time.Now().UTC()
	tokenHash := hashBindingToken(rawToken)
	token, ok, err := s.tokens.FindActiveByHash(ctx, tokenHash, now)
	if err != nil || !ok {
		return 0, ok, err
	}
	if err := s.accounts.BindTelegramChat(ctx, token.UserID, chatID); err != nil {
		return 0, false, err
	}
	if err := s.tokens.MarkUsed(ctx, tokenHash, now); err != nil {
		return 0, false, err
	}
	return token.UserID, true, nil
}

// randomBindingToken creates a URL-safe binding token with 32 bytes of entropy.
//
// Author: monsterfei
// Date: 2026-07-01
// @returns Raw binding token.
func randomBindingToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// hashBindingToken hashes a raw binding token for storage lookup.
//
// Author: monsterfei
// Date: 2026-07-01
// @param rawToken Raw binding token.
// @returns SHA-256 hex hash.
func hashBindingToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
