package auth

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

const (
	defaultSessionTTL       = 168 * time.Hour
	defaultPasswordResetTTL = 30 * time.Minute
)

// Config controls account-session token lifetimes.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type Config struct {
	SessionTTL       time.Duration
	PasswordResetTTL time.Duration
	ExposeResetToken bool
}

// RegisterRequest contains one account registration payload.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type RegisterRequest struct {
	Email    string
	Password string
}

// LoginRequest contains one account login payload.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type LoginRequest struct {
	Email    string
	Password string
}

// AuthSession returns a safe user record and raw session token.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type AuthSession struct {
	User      model.User
	Token     string
	ExpiresAt time.Time
}

// PlanLimits contains backend subscription entitlements.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type PlanLimits struct {
	MaxRules     int `json:"max_rules"`
	AlertHistory int `json:"alert_history"`
}

// UserRepository defines persisted account operations required by auth.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type UserRepository interface {
	CreateWithPassword(context.Context, model.User) (model.User, error)
	FindByEmail(context.Context, string) (model.User, bool, error)
	FindByID(context.Context, int64) (model.User, bool, error)
	UpdatePassword(context.Context, int64, string) error
}

// SessionRepository defines persisted session operations required by auth.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type SessionRepository interface {
	Create(context.Context, model.UserSession) error
	FindActiveByHash(context.Context, string, time.Time) (model.UserSession, bool, error)
	RevokeByHash(context.Context, string, time.Time) error
}

// PasswordResetRepository defines persisted password reset token operations.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type PasswordResetRepository interface {
	Create(context.Context, model.PasswordResetToken) error
	FindActiveByHash(context.Context, string, time.Time) (model.PasswordResetToken, bool, error)
	MarkUsed(context.Context, string, time.Time) error
}

// Service coordinates account auth, sessions, and password resets.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type Service struct {
	users       UserRepository
	sessions    SessionRepository
	resetTokens PasswordResetRepository
	cfg         Config
}

// NewService creates an auth service from narrow persistence interfaces.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param users User account repository.
// @param sessions User session repository.
// @param resetTokens Password reset token repository.
// @param cfg Auth service configuration.
// @returns Auth service.
func NewService(users UserRepository, sessions SessionRepository, resetTokens PasswordResetRepository, cfg Config) Service {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = defaultSessionTTL
	}
	if cfg.PasswordResetTTL == 0 {
		cfg.PasswordResetTTL = defaultPasswordResetTTL
	}
	return Service{
		users:       users,
		sessions:    sessions,
		resetTokens: resetTokens,
		cfg:         cfg,
	}
}

// Register creates one active free-plan user and login session.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param req Registration payload.
// @returns Auth session containing raw token for cookie setting.
func (s Service) Register(ctx context.Context, req RegisterRequest) (AuthSession, error) {
	if s.users == nil || s.sessions == nil {
		return AuthSession{}, errors.New("auth repositories are not configured")
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return AuthSession{}, err
	}
	if _, ok, err := s.users.FindByEmail(ctx, email); err != nil {
		return AuthSession{}, err
	} else if ok {
		return AuthSession{}, errors.New("email already registered")
	}
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return AuthSession{}, err
	}
	now := time.Now().UTC()
	user, err := s.users.CreateWithPassword(ctx, model.User{
		Email:        email,
		PasswordHash: passwordHash,
		Plan:         model.UserPlanFree,
		Status:       model.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return AuthSession{}, err
	}
	return s.createSession(ctx, user)
}

// Login validates credentials and creates a new login session.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param req Login payload.
// @returns Auth session containing raw token for cookie setting.
func (s Service) Login(ctx context.Context, req LoginRequest) (AuthSession, error) {
	if s.users == nil || s.sessions == nil {
		return AuthSession{}, errors.New("auth repositories are not configured")
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return AuthSession{}, err
	}
	user, ok, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return AuthSession{}, err
	}
	if !ok || user.PasswordHash == "" {
		return AuthSession{}, errors.New("invalid email or password")
	}
	if user.Status == model.UserStatusDisabled {
		return AuthSession{}, errors.New("user is disabled")
	}
	if err := CheckPassword(user.PasswordHash, req.Password); err != nil {
		return AuthSession{}, errors.New("invalid email or password")
	}
	return s.createSession(ctx, user)
}

// Logout revokes a session token if it exists.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param token Raw session token.
// @returns Error when session persistence fails.
func (s Service) Logout(ctx context.Context, token string) error {
	if s.sessions == nil {
		return errors.New("session repository is not configured")
	}
	if token == "" {
		return nil
	}
	return s.sessions.RevokeByHash(ctx, hashToken(token), time.Now().UTC())
}

// CurrentUser resolves a raw session token to an active user.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param token Raw session token.
// @returns User, whether the token is valid, and lookup error.
func (s Service) CurrentUser(ctx context.Context, token string) (model.User, bool, error) {
	if s.users == nil || s.sessions == nil {
		return model.User{}, false, errors.New("auth repositories are not configured")
	}
	if token == "" {
		return model.User{}, false, nil
	}
	session, ok, err := s.sessions.FindActiveByHash(ctx, hashToken(token), time.Now().UTC())
	if err != nil || !ok {
		return model.User{}, false, err
	}
	user, ok, err := s.users.FindByID(ctx, session.UserID)
	if err != nil || !ok {
		return model.User{}, ok, err
	}
	if user.Status == model.UserStatusDisabled {
		return model.User{}, false, nil
	}
	return user, true, nil
}

// RequestPasswordReset creates an expiring reset token for an existing user.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param email Account email address.
// @returns Raw reset token only when development exposure is enabled.
func (s Service) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	if s.users == nil || s.resetTokens == nil {
		return "", errors.New("auth repositories are not configured")
	}
	normalized, err := normalizeEmail(email)
	if err != nil {
		return "", nil
	}
	user, ok, err := s.users.FindByEmail(ctx, normalized)
	if err != nil || !ok {
		return "", err
	}
	rawToken, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	if err := s.resetTokens.Create(ctx, model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashToken(rawToken),
		ExpiresAt: now.Add(s.cfg.PasswordResetTTL),
		CreatedAt: now,
	}); err != nil {
		return "", err
	}
	if !s.cfg.ExposeResetToken {
		return "", nil
	}
	return rawToken, nil
}

// ConfirmPasswordReset consumes one reset token and updates the password.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param token Raw reset token.
// @param newPassword New strong password.
// @returns Error when the token is invalid or the update fails.
func (s Service) ConfirmPasswordReset(ctx context.Context, token string, newPassword string) error {
	if s.users == nil || s.resetTokens == nil {
		return errors.New("auth repositories are not configured")
	}
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	tokenHash := hashToken(token)
	resetToken, ok, err := s.resetTokens.FindActiveByHash(ctx, tokenHash, time.Now().UTC())
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("invalid or expired reset token")
	}
	if err := s.users.UpdatePassword(ctx, resetToken.UserID, passwordHash); err != nil {
		return err
	}
	return s.resetTokens.MarkUsed(ctx, tokenHash, time.Now().UTC())
}

// ChangePassword changes a user's password after checking the current password.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id.
// @param currentPassword Current raw password.
// @param newPassword New strong password.
// @returns Error when validation or persistence fails.
func (s Service) ChangePassword(ctx context.Context, userID int64, currentPassword string, newPassword string) error {
	if s.users == nil {
		return errors.New("user repository is not configured")
	}
	user, ok, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !ok || user.PasswordHash == "" {
		return errors.New("user not found")
	}
	if err := CheckPassword(user.PasswordHash, currentPassword); err != nil {
		return errors.New("invalid current password")
	}
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.users.UpdatePassword(ctx, userID, passwordHash)
}

// LimitsForPlan returns bounded entitlements for a subscription plan.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param plan Subscription plan.
// @returns Backend rule and alert-history limits.
func LimitsForPlan(plan string) PlanLimits {
	switch plan {
	case model.UserPlanPro:
		return PlanLimits{MaxRules: 50, AlertHistory: 100}
	case model.UserPlanVIP:
		return PlanLimits{MaxRules: 200, AlertHistory: 200}
	default:
		return PlanLimits{MaxRules: 5, AlertHistory: 20}
	}
}

// createSession persists one session and returns its raw token to the caller.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param ctx Request context.
// @param user User to create a session for.
// @returns Auth session with raw token.
func (s Service) createSession(ctx context.Context, user model.User) (AuthSession, error) {
	rawToken, err := randomToken()
	if err != nil {
		return AuthSession{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.SessionTTL)
	if err := s.sessions.Create(ctx, model.UserSession{
		UserID:    user.ID,
		TokenHash: hashToken(rawToken),
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return AuthSession{}, err
	}
	return AuthSession{User: user, Token: rawToken, ExpiresAt: expiresAt}, nil
}

// normalizeEmail canonicalizes and lightly validates an email address.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param email Raw email.
// @returns Lower-cased email or validation error.
func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" || !strings.Contains(normalized, "@") {
		return "", errors.New("valid email is required")
	}
	return normalized, nil
}

// randomToken creates a URL-safe raw token with 32 bytes of entropy.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @returns Raw token string.
func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// hashToken hashes a raw token for storage lookup.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param token Raw token.
// @returns SHA-256 hex hash.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
