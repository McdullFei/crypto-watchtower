package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// TestValidateStrongPasswordRejectsWeakPasswords verifies weak account passwords are rejected.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestValidateStrongPasswordRejectsWeakPasswords(t *testing.T) {
	weakPasswords := []string{
		"Short1!",
		"lowercase1!",
		"UPPERCASE1!",
		"NoNumber!",
		"NoSpecial1",
	}
	for _, password := range weakPasswords {
		if err := ValidateStrongPassword(password); err == nil {
			t.Fatalf("expected weak password %q to be rejected", password)
		}
	}
}

// TestValidateStrongPasswordAcceptsComplexPassword verifies complex account passwords pass validation.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestValidateStrongPasswordAcceptsComplexPassword(t *testing.T) {
	if err := ValidateStrongPassword("Strong1!"); err != nil {
		t.Fatalf("expected strong password: %v", err)
	}
}

// memoryUsers stores account records for auth service tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type memoryUsers struct {
	nextID  int64
	byID    map[int64]model.User
	byEmail map[string]int64
}

// memorySessions stores session records for auth service tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type memorySessions struct {
	byHash map[string]model.UserSession
}

// memoryResetTokens stores password reset records for auth service tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
type memoryResetTokens struct {
	byHash map[string]model.PasswordResetToken
}

// newMemoryUsers creates an empty in-memory user repository for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func newMemoryUsers() *memoryUsers {
	return &memoryUsers{
		nextID:  1,
		byID:    map[int64]model.User{},
		byEmail: map[string]int64{},
	}
}

// newMemorySessions creates an empty in-memory session repository for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func newMemorySessions() *memorySessions {
	return &memorySessions{byHash: map[string]model.UserSession{}}
}

// newMemoryResetTokens creates an empty in-memory password reset repository for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func newMemoryResetTokens() *memoryResetTokens {
	return &memoryResetTokens{byHash: map[string]model.PasswordResetToken{}}
}

// CreateWithPassword stores one user with a password hash for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memoryUsers) CreateWithPassword(_ context.Context, user model.User) (model.User, error) {
	emailKey := strings.ToLower(user.Email)
	if _, ok := m.byEmail[emailKey]; ok {
		return model.User{}, errors.New("duplicate email")
	}
	user.ID = m.nextID
	m.nextID++
	m.byID[user.ID] = user
	m.byEmail[emailKey] = user.ID
	return user, nil
}

// FindByEmail returns one user by normalized email for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memoryUsers) FindByEmail(_ context.Context, email string) (model.User, bool, error) {
	id, ok := m.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return model.User{}, false, nil
	}
	return m.byID[id], true, nil
}

// FindByID returns one user by id for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memoryUsers) FindByID(_ context.Context, userID int64) (model.User, bool, error) {
	user, ok := m.byID[userID]
	return user, ok, nil
}

// UpdatePassword updates one user's password hash for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memoryUsers) UpdatePassword(_ context.Context, userID int64, passwordHash string) error {
	user, ok := m.byID[userID]
	if !ok {
		return errors.New("user not found")
	}
	user.PasswordHash = passwordHash
	m.byID[userID] = user
	return nil
}

// Create stores one session for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memorySessions) Create(_ context.Context, session model.UserSession) error {
	m.byHash[session.TokenHash] = session
	return nil
}

// FindActiveByHash returns one active session for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memorySessions) FindActiveByHash(_ context.Context, tokenHash string, now time.Time) (model.UserSession, bool, error) {
	session, ok := m.byHash[tokenHash]
	if !ok || session.RevokedAt != nil || !session.ExpiresAt.After(now) {
		return model.UserSession{}, false, nil
	}
	return session, true, nil
}

// RevokeByHash revokes one session for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memorySessions) RevokeByHash(_ context.Context, tokenHash string, now time.Time) error {
	session, ok := m.byHash[tokenHash]
	if !ok {
		return nil
	}
	session.RevokedAt = &now
	m.byHash[tokenHash] = session
	return nil
}

// Create stores one password reset token for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memoryResetTokens) Create(_ context.Context, token model.PasswordResetToken) error {
	m.byHash[token.TokenHash] = token
	return nil
}

// FindActiveByHash returns one active password reset token for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memoryResetTokens) FindActiveByHash(_ context.Context, tokenHash string, now time.Time) (model.PasswordResetToken, bool, error) {
	token, ok := m.byHash[tokenHash]
	if !ok || token.UsedAt != nil || !token.ExpiresAt.After(now) {
		return model.PasswordResetToken{}, false, nil
	}
	return token, true, nil
}

// MarkUsed consumes one password reset token for auth tests.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func (m *memoryResetTokens) MarkUsed(_ context.Context, tokenHash string, now time.Time) error {
	token, ok := m.byHash[tokenHash]
	if !ok {
		return nil
	}
	token.UsedAt = &now
	m.byHash[tokenHash] = token
	return nil
}

// TestServiceRegisterCreatesUserAndSession verifies registration stores a hashed password and session hash.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestServiceRegisterCreatesUserAndSession(t *testing.T) {
	users := newMemoryUsers()
	sessions := newMemorySessions()
	service := NewService(users, sessions, newMemoryResetTokens(), Config{SessionTTL: time.Hour, PasswordResetTTL: time.Hour})

	session, err := service.Register(context.Background(), RegisterRequest{
		Email:    " User@Example.COM ",
		Password: "Strong1!",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if session.Token == "" {
		t.Fatal("expected raw session token")
	}
	if session.User.Email != "user@example.com" || session.User.Plan != model.UserPlanFree || session.User.Status != model.UserStatusActive {
		t.Fatalf("unexpected registered user: %+v", session.User)
	}
	if session.User.PasswordHash == "" || session.User.PasswordHash == "Strong1!" {
		t.Fatalf("expected bcrypt password hash, got %q", session.User.PasswordHash)
	}
	if _, ok := sessions.byHash[session.Token]; ok {
		t.Fatal("raw session token must not be stored")
	}
	if len(sessions.byHash) != 1 {
		t.Fatalf("expected one stored session, got %d", len(sessions.byHash))
	}
	for tokenHash := range sessions.byHash {
		if len(tokenHash) != 64 {
			t.Fatalf("expected sha256 token hash, got %q", tokenHash)
		}
	}
	if _, err := service.Register(context.Background(), RegisterRequest{Email: "user@example.com", Password: "Strong1!"}); err == nil {
		t.Fatal("expected duplicate email to fail")
	}
}

// TestServiceLoginRejectsWrongPassword verifies login rejects invalid credentials.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestServiceLoginRejectsWrongPassword(t *testing.T) {
	service := NewService(newMemoryUsers(), newMemorySessions(), newMemoryResetTokens(), Config{SessionTTL: time.Hour, PasswordResetTTL: time.Hour})
	if _, err := service.Register(context.Background(), RegisterRequest{Email: "user@example.com", Password: "Strong1!"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := service.Login(context.Background(), LoginRequest{Email: "user@example.com", Password: "Wrong1!"}); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

// TestServiceLoginCreatesSessionForCorrectPassword verifies login returns a usable session token.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestServiceLoginCreatesSessionForCorrectPassword(t *testing.T) {
	users := newMemoryUsers()
	sessions := newMemorySessions()
	service := NewService(users, sessions, newMemoryResetTokens(), Config{SessionTTL: time.Hour, PasswordResetTTL: time.Hour})
	if _, err := service.Register(context.Background(), RegisterRequest{Email: "user@example.com", Password: "Strong1!"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	session, err := service.Login(context.Background(), LoginRequest{Email: " USER@example.com ", Password: "Strong1!"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if session.Token == "" || session.User.Email != "user@example.com" {
		t.Fatalf("unexpected session: %+v", session)
	}
	current, ok, err := service.CurrentUser(context.Background(), session.Token)
	if err != nil || !ok || current.ID != session.User.ID {
		t.Fatalf("current user: user=%+v ok=%v err=%v", current, ok, err)
	}
}

// TestServiceLogoutRevokesSession verifies logout invalidates the current session token.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestServiceLogoutRevokesSession(t *testing.T) {
	sessions := newMemorySessions()
	service := NewService(newMemoryUsers(), sessions, newMemoryResetTokens(), Config{SessionTTL: time.Hour, PasswordResetTTL: time.Hour})
	session, err := service.Register(context.Background(), RegisterRequest{Email: "user@example.com", Password: "Strong1!"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := service.Logout(context.Background(), session.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, ok, err := service.CurrentUser(context.Background(), session.Token); err != nil || ok {
		t.Fatalf("expected revoked session, ok=%v err=%v", ok, err)
	}
}

// TestServicePasswordResetUsesOneTimeToken verifies reset tokens are expiring one-time credentials.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestServicePasswordResetUsesOneTimeToken(t *testing.T) {
	service := NewService(newMemoryUsers(), newMemorySessions(), newMemoryResetTokens(), Config{
		SessionTTL:       time.Hour,
		PasswordResetTTL: time.Hour,
		ExposeResetToken: true,
	})
	if _, err := service.Register(context.Background(), RegisterRequest{Email: "user@example.com", Password: "Strong1!"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	token, err := service.RequestPasswordReset(context.Background(), "USER@example.com")
	if err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if token == "" {
		t.Fatal("expected reset token in dev mode")
	}
	if err := service.ConfirmPasswordReset(context.Background(), token, "Better1!"); err != nil {
		t.Fatalf("confirm reset: %v", err)
	}
	if err := service.ConfirmPasswordReset(context.Background(), token, "Better1!"); err == nil {
		t.Fatal("expected reset token reuse to fail")
	}
	if _, err := service.Login(context.Background(), LoginRequest{Email: "user@example.com", Password: "Better1!"}); err != nil {
		t.Fatalf("login with reset password: %v", err)
	}
}

// TestServiceChangePasswordRequiresCurrentPassword verifies password changes require the current password.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestServiceChangePasswordRequiresCurrentPassword(t *testing.T) {
	service := NewService(newMemoryUsers(), newMemorySessions(), newMemoryResetTokens(), Config{SessionTTL: time.Hour, PasswordResetTTL: time.Hour})
	session, err := service.Register(context.Background(), RegisterRequest{Email: "user@example.com", Password: "Strong1!"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := service.ChangePassword(context.Background(), session.User.ID, "Wrong1!", "Better1!"); err == nil {
		t.Fatal("expected wrong current password to fail")
	}
	if err := service.ChangePassword(context.Background(), session.User.ID, "Strong1!", "weak"); err == nil {
		t.Fatal("expected weak new password to fail")
	}
	if err := service.ChangePassword(context.Background(), session.User.ID, "Strong1!", "Better1!"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := service.Login(context.Background(), LoginRequest{Email: "user@example.com", Password: "Better1!"}); err != nil {
		t.Fatalf("login with changed password: %v", err)
	}
}

// TestPlanLimitsReturnBoundedEntitlements verifies subscription plans map to bounded limits.
//
// Author: __AUTHOR__
// Date: 2026-06-30
func TestPlanLimitsReturnBoundedEntitlements(t *testing.T) {
	cases := []struct {
		plan    string
		rules   int
		history int
	}{
		{plan: model.UserPlanFree, rules: 5, history: 20},
		{plan: model.UserPlanPro, rules: 50, history: 100},
		{plan: model.UserPlanVIP, rules: 200, history: 200},
		{plan: "unknown", rules: 5, history: 20},
	}
	for _, tc := range cases {
		limits := LimitsForPlan(tc.plan)
		if limits.MaxRules != tc.rules || limits.AlertHistory != tc.history {
			t.Fatalf("unexpected limits for %s: %+v", tc.plan, limits)
		}
	}
}
