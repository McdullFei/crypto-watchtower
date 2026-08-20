# SaaS Auth And Subscription Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first SaaS authentication and subscription foundation so user dashboard actions are tied to a real logged-in user instead of Bearer Token plus explicit `user_id`.

**Architecture:** Keep operator APIs on the existing Bearer Token guard. Add session-cookie authentication for `/api/v1/auth/*` and `/api/v1/user/*`, backed by PostgreSQL user/session/reset-token tables and bcrypt password hashes. Subscription support is a local plan/status entitlement layer (`free`, `pro`, `vip`) with rule-count and history-limit enforcement; payment provider integration is outside this plan.

**Tech Stack:** Go 1.24, standard `net/http`, PostgreSQL 16.14 migrations/repositories, `golang.org/x/crypto/bcrypt`, secure random tokens from `crypto/rand`, embedded HTML/CSS/JavaScript dashboard, Docker Compose smoke.

---

## Acceptance Criteria

- [x] Users can register with email and strong password through `POST /api/v1/auth/register`.
- [x] Users can log in through `POST /api/v1/auth/login` and receive an HttpOnly session cookie.
- [x] Users can log out through `POST /api/v1/auth/logout`; the session is revoked.
- [x] Password complexity is enforced on register, password reset, and password change: at least 8 chars, uppercase, lowercase, digit, special character.
- [x] Password hashes are stored with bcrypt; raw passwords are never persisted or returned.
- [x] Password reset request and confirm APIs exist and use expiring one-time reset tokens.
- [x] `/api/v1/user/profile`, `/api/v1/user/rules`, and `/api/v1/user/alerts` derive the current user from the session cookie and no longer require `user_id`.
- [x] Operator Bearer Token APIs under `/api/v1/admin/*` and `/api/v1/rules` keep their current behavior.
- [x] User profile exposes `plan`, `status`, and bounded limits without exposing password hash, raw session token, or raw reset token in production mode.
- [x] Free/Pro/VIP rule-count and alert-history limits are enforced in the backend.
- [x] `/dashboard` provides login/register/logout/password controls and no longer asks for Bearer Token or manual User ID.
- [x] README, user manual, and master plan describe the auth/session boundary and the fact that payment billing is still not included.
- [x] Verification gate passes.

## Dependency

- [x] Complete and verify `[7]2026-06-30-user-dashboard.md` before starting this plan.

## Scope Boundary

- This plan includes account auth, sessions, password reset/change, local subscription plans, and Dashboard migration.
- This plan does not include Stripe/Alipay/WeChat billing, invoice generation, organization/team accounts, OAuth, email delivery infrastructure, or multi-device session management UI.
- Reset-token delivery is local-development friendly: when `app.env != "prod"`, the request API may return the reset token for manual testing; when `app.env == "prod"`, it returns only a generic accepted response.

## File Structure

- Create: `migrations/004_auth_subscription.sql` - add auth columns, session table, reset token table, and lookup indexes.
- Modify: `internal/model/user.go` - add password hash, plan/status constants, and session/reset token models.
- Create: `internal/auth/password.go` - strong password validation and bcrypt hashing helpers.
- Create: `internal/auth/service.go` - register, login, logout, reset request/confirm, password change, current-session lookup, plan limits.
- Create: `internal/auth/service_test.go` - auth service unit tests.
- Modify: `internal/storage/user_repo.go` - email lookup, user create/update password, plan/status read.
- Create: `internal/storage/session_repo.go` - persisted user sessions.
- Create: `internal/storage/password_reset_repo.go` - expiring one-time password reset tokens.
- Modify: `internal/storage/repositories.go` - expose new repositories.
- Create: `internal/api/auth.go` - auth API routes, cookie helpers, session middleware helpers.
- Create: `internal/api/auth_test.go` - auth API behavior tests.
- Modify: `internal/api/user.go` - derive user id from session for user profile/rules/alerts; keep helper path for internal tests only.
- Modify: `internal/api/user_test.go` - update user API tests from Bearer/user_id to session-cookie auth.
- Modify: `internal/api/router.go` - add auth dependency and mount auth routes.
- Modify: `cmd/server/main.go` - wire auth service and repositories.
- Modify: `internal/api/dashboardui/index.html` - add auth controls and remove Bearer/User ID controls.
- Modify: `internal/api/dashboardui/app.js` - call auth APIs with session cookie and remove manual `user_id`.
- Modify: `internal/api/dashboardui/styles.css` - support compact auth panels.
- Modify: `README.md`, `docs/用户手册.md`, `docs/plan/币圈异动监控平台总体开发计划.md` - document new flow and update plan status.

## Task 1: Auth Schema And Models

**Files:**
- Create: `migrations/004_auth_subscription.sql`
- Modify: `internal/model/user.go`
- Modify: `internal/storage/repositories.go`
- Test: `internal/storage/migration_test.go`

- [x] **Step 1: Write the failing migration content test**

Add this test to `internal/storage/migration_test.go`:

```go
// TestAuthMigrationAddsSessionAndPasswordTables verifies auth schema changes are present.
//
// Author: monsterfei
// Date: 2026-06-30
func TestAuthMigrationAddsSessionAndPasswordTables(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/004_auth_subscription.sql")
	if err != nil {
		t.Fatalf("read auth migration: %v", err)
	}
	sql := string(raw)
	for _, expected := range []string{
		"ADD COLUMN IF NOT EXISTS password_hash",
		"CREATE TABLE IF NOT EXISTS user_sessions",
		"CREATE TABLE IF NOT EXISTS password_reset_tokens",
		"idx_users_email_lower",
		"idx_user_sessions_token_hash",
		"idx_password_reset_tokens_token_hash",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected %q in auth migration", expected)
		}
	}
}
```

Also add imports if missing:

```go
import (
	"os"
	"strings"
)
```

- [x] **Step 2: Run the migration test and verify RED**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/storage -run AuthMigration -v'
```

Expected: FAIL because `migrations/004_auth_subscription.sql` does not exist.

- [x] **Step 3: Add auth migration**

Create `migrations/004_auth_subscription.sql`:

```sql
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash TEXT,
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower
    ON users (LOWER(email))
    WHERE email IS NOT NULL;

CREATE TABLE IF NOT EXISTS user_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_sessions_token_hash
    ON user_sessions(token_hash);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_expires
    ON user_sessions(user_id, expires_at DESC);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash
    ON password_reset_tokens(token_hash);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_expires
    ON password_reset_tokens(user_id, expires_at DESC);
```

- [x] **Step 4: Extend user models**

Modify `internal/model/user.go`:

```go
package model

import "time"

const (
	UserPlanFree = "free"
	UserPlanPro  = "pro"
	UserPlanVIP  = "vip"

	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

// User stores account, notification, and subscription metadata.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-06-30
type User struct {
	ID             int64
	Email          string
	PasswordHash   string
	EmailVerified  bool
	TelegramChatID string
	Plan           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UserSession stores one persisted login session.
//
// Author: monsterfei
// Date: 2026-06-30
type UserSession struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PasswordResetToken stores one expiring password reset token.
//
// Author: monsterfei
// Date: 2026-06-30
type PasswordResetToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
```

- [x] **Step 5: Run schema/model tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/gofmt -w internal/model/user.go internal/storage/migration_test.go && /usr/local/go/bin/go test ./internal/model ./internal/storage -run AuthMigration -v'
```

Expected: PASS.

## Task 2: Password Policy And Auth Service Core

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`
- Modify: `go.mod`

- [x] **Step 1: Write failing password policy tests**

Create `internal/auth/service_test.go` with:

```go
package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

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

func TestValidateStrongPasswordAcceptsComplexPassword(t *testing.T) {
	if err := ValidateStrongPassword("Strong1!"); err != nil {
		t.Fatalf("expected strong password: %v", err)
	}
}
```

- [x] **Step 2: Run password policy tests and verify RED**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/auth -run ValidateStrongPassword -v'
```

Expected: FAIL because `internal/auth` and `ValidateStrongPassword` do not exist.

- [x] **Step 3: Implement password policy and bcrypt helpers**

Create `internal/auth/password.go`:

```go
package auth

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// ValidateStrongPassword enforces the account password complexity policy.
//
// Author: monsterfei
// Date: 2026-06-30
// @param password Raw password to validate.
// @returns Error when the password is weak.
func ValidateStrongPassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return errors.New("password must include uppercase, lowercase, digit, and special characters")
	}
	if strings.ContainsAny(password, " \t\r\n") {
		return errors.New("password must not contain whitespace")
	}
	return nil
}

// HashPassword hashes a validated raw password with bcrypt.
//
// Author: monsterfei
// Date: 2026-06-30
// @param password Raw password.
// @returns Bcrypt password hash.
func HashPassword(password string) (string, error) {
	if err := ValidateStrongPassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a raw password against a bcrypt hash.
//
// Author: monsterfei
// Date: 2026-06-30
// @param hash Stored bcrypt hash.
// @param password Raw password.
// @returns Error when the password does not match.
func CheckPassword(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
```

Move `golang.org/x/crypto` from indirect to direct in `go.mod` by running:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go mod tidy'
```

- [x] **Step 4: Write failing register/login/reset/change service tests**

Extend `internal/auth/service_test.go` with focused in-memory repositories:

```go
type memoryUsers struct {
	nextID int64
	byID  map[int64]model.User
	byEmail map[string]int64
}

type memorySessions struct {
	byHash map[string]model.UserSession
}

type memoryResetTokens struct {
	byHash map[string]model.PasswordResetToken
}
```

Add these tests:

```go
func TestServiceRegisterCreatesUserAndSession(t *testing.T)
func TestServiceLoginRejectsWrongPassword(t *testing.T)
func TestServiceLoginCreatesSessionForCorrectPassword(t *testing.T)
func TestServiceLogoutRevokesSession(t *testing.T)
func TestServicePasswordResetUsesOneTimeToken(t *testing.T)
func TestServiceChangePasswordRequiresCurrentPassword(t *testing.T)
func TestPlanLimitsReturnBoundedEntitlements(t *testing.T)
```

Expected behavior:

- Register lowercases and trims email.
- Duplicate email returns an error.
- Register/Login returns a raw session token once; repository stores only token hash.
- Logout sets `revoked_at`.
- Reset token expires and can only be consumed once.
- Password change rejects weak new passwords and wrong current password.
- `free`, `pro`, and `vip` plans return deterministic rule/history limits.

- [x] **Step 5: Run auth service tests and verify RED**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/auth -run Service -v'
```

Expected: FAIL because `Service` and repository interfaces are not implemented.

- [x] **Step 6: Implement auth service**

Create `internal/auth/service.go` with these exported types and methods:

```go
type Config struct {
	SessionTTL          time.Duration
	PasswordResetTTL    time.Duration
	ExposeResetToken    bool
}

type RegisterRequest struct {
	Email    string
	Password string
}

type LoginRequest struct {
	Email    string
	Password string
}

type AuthSession struct {
	User  model.User
	Token string
	ExpiresAt time.Time
}

type PlanLimits struct {
	MaxRules       int `json:"max_rules"`
	AlertHistory  int `json:"alert_history"`
}

type UserRepository interface {
	CreateWithPassword(context.Context, model.User) (model.User, error)
	FindByEmail(context.Context, string) (model.User, bool, error)
	FindByID(context.Context, int64) (model.User, bool, error)
	UpdatePassword(context.Context, int64, string) error
}

type SessionRepository interface {
	Create(context.Context, model.UserSession) error
	FindActiveByHash(context.Context, string, time.Time) (model.UserSession, bool, error)
	RevokeByHash(context.Context, string, time.Time) error
}

type PasswordResetRepository interface {
	Create(context.Context, model.PasswordResetToken) error
	FindActiveByHash(context.Context, string, time.Time) (model.PasswordResetToken, bool, error)
	MarkUsed(context.Context, string, time.Time) error
}
```

Implement:

```go
func NewService(users UserRepository, sessions SessionRepository, resetTokens PasswordResetRepository, cfg Config) Service
func (s Service) Register(ctx context.Context, req RegisterRequest) (AuthSession, error)
func (s Service) Login(ctx context.Context, req LoginRequest) (AuthSession, error)
func (s Service) Logout(ctx context.Context, token string) error
func (s Service) CurrentUser(ctx context.Context, token string) (model.User, bool, error)
func (s Service) RequestPasswordReset(ctx context.Context, email string) (string, error)
func (s Service) ConfirmPasswordReset(ctx context.Context, token string, newPassword string) error
func (s Service) ChangePassword(ctx context.Context, userID int64, currentPassword string, newPassword string) error
func LimitsForPlan(plan string) PlanLimits
```

Use `crypto/rand` to create at least 32 bytes of raw token entropy, store only `sha256` hex hashes in repositories, and return raw tokens only to the API/cookie layer.

- [x] **Step 7: Run auth service tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/gofmt -w internal/auth/*.go && /usr/local/go/bin/go test ./internal/auth -v'
```

Expected: PASS.

## Task 3: Auth Storage Repositories

**Files:**
- Modify: `internal/storage/user_repo.go`
- Create: `internal/storage/session_repo.go`
- Create: `internal/storage/password_reset_repo.go`
- Modify: `internal/storage/repositories.go`
- Modify: `internal/integration/postgres_redis_test.go`

- [x] **Step 1: Write failing repository integration test**

Add to `internal/integration/postgres_redis_test.go`:

```go
// TestAuthRepositoriesPersistSessionsAndResetTokens verifies auth persistence against PostgreSQL.
//
// Author: monsterfei
// Date: 2026-06-30
func TestAuthRepositoriesPersistSessionsAndResetTokens(t *testing.T) {
	ctx := context.Background()
	repos := setupIntegrationRepositories(t)
	user, err := repos.Users.CreateWithPassword(ctx, model.User{
		Email:        "auth-user@example.com",
		PasswordHash: "bcrypt-hash",
		Plan:         model.UserPlanFree,
		Status:       model.UserStatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	found, ok, err := repos.Users.FindByEmail(ctx, "auth-user@example.com")
	if err != nil || !ok || found.ID != user.ID {
		t.Fatalf("find user by email: user=%+v ok=%v err=%v", found, ok, err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if err := repos.Sessions.Create(ctx, model.UserSession{UserID: user.ID, TokenHash: strings.Repeat("a", 64), ExpiresAt: expiresAt, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	session, ok, err := repos.Sessions.FindActiveByHash(ctx, strings.Repeat("a", 64), time.Now().UTC())
	if err != nil || !ok || session.UserID != user.ID {
		t.Fatalf("find active session: session=%+v ok=%v err=%v", session, ok, err)
	}
	if err := repos.PasswordResetTokens.Create(ctx, model.PasswordResetToken{UserID: user.ID, TokenHash: strings.Repeat("b", 64), ExpiresAt: expiresAt, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create reset token: %v", err)
	}
	resetToken, ok, err := repos.PasswordResetTokens.FindActiveByHash(ctx, strings.Repeat("b", 64), time.Now().UTC())
	if err != nil || !ok || resetToken.UserID != user.ID {
		t.Fatalf("find active reset token: token=%+v ok=%v err=%v", resetToken, ok, err)
	}
}
```

- [x] **Step 2: Run repository test and verify RED**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/integration -run AuthRepositories -v'
```

Expected: FAIL because auth repository methods do not exist.

- [x] **Step 3: Implement user repository auth methods**

Add to `internal/storage/user_repo.go`:

```go
func (r UserRepo) CreateWithPassword(ctx context.Context, user model.User) (model.User, error)
func (r UserRepo) FindByEmail(ctx context.Context, email string) (model.User, bool, error)
func (r UserRepo) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error
```

All queries must select bounded single rows and use `LOWER(email) = LOWER($1)`.

- [x] **Step 4: Implement session and reset repositories**

Create `internal/storage/session_repo.go`:

```go
type SessionRepo struct {
	DB *pgxpool.Pool
}

func (r SessionRepo) Create(ctx context.Context, session model.UserSession) error
func (r SessionRepo) FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.UserSession, bool, error)
func (r SessionRepo) RevokeByHash(ctx context.Context, tokenHash string, now time.Time) error
```

Create `internal/storage/password_reset_repo.go`:

```go
type PasswordResetRepo struct {
	DB *pgxpool.Pool
}

func (r PasswordResetRepo) Create(ctx context.Context, token model.PasswordResetToken) error
func (r PasswordResetRepo) FindActiveByHash(ctx context.Context, tokenHash string, now time.Time) (model.PasswordResetToken, bool, error)
func (r PasswordResetRepo) MarkUsed(ctx context.Context, tokenHash string, now time.Time) error
```

- [x] **Step 5: Wire repositories**

Modify `internal/storage/repositories.go`:

```go
type Repositories struct {
	MarketEvents        MarketEventRepo
	MarketSummaries     MarketSummaryRepo
	AlertRules          AlertRuleRepo
	Alerts              AlertRepo
	NotificationLogs    NotificationLogRepo
	Users               UserRepo
	Sessions            SessionRepo
	PasswordResetTokens PasswordResetRepo
}
```

Initialize `Sessions` and `PasswordResetTokens` in `NewRepositories`.

- [x] **Step 6: Run repository integration test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/gofmt -w internal/storage/*.go internal/integration/postgres_redis_test.go && /usr/local/go/bin/go test ./internal/integration -run AuthRepositories -v'
```

Expected: PASS.

## Task 4: Auth API And Session Cookie

**Files:**
- Create: `internal/api/auth.go`
- Create: `internal/api/auth_test.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/server/main.go`
- Modify: `internal/config/config.go`
- Modify: `configs/config.example.yaml`

- [x] **Step 1: Write failing Auth API tests**

Create `internal/api/auth_test.go` with tests:

```go
func TestAuthRegisterRejectsWeakPassword(t *testing.T)
func TestAuthRegisterSetsSessionCookie(t *testing.T)
func TestAuthLoginSetsSessionCookie(t *testing.T)
func TestAuthLogoutClearsSessionCookie(t *testing.T)
func TestPasswordResetRequestReturnsGenericResponse(t *testing.T)
func TestPasswordResetConfirmRejectsWeakPassword(t *testing.T)
func TestChangePasswordRequiresSession(t *testing.T)
```

Expected behavior:

- Weak password returns HTTP `400`.
- Register/Login set `Set-Cookie: cw_session=...; HttpOnly; SameSite=Lax`.
- Logout revokes server session and sets an expired cookie.
- Reset request does not reveal whether email exists.
- Change password without a valid session returns `401`.

- [x] **Step 2: Run Auth API tests and verify RED**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run Auth -v'
```

Expected: FAIL because auth routes and API dependency do not exist.

- [x] **Step 3: Add auth config**

Modify `internal/config/config.go`:

```go
Auth AuthConfig `yaml:"auth"`

// AuthConfig contains account-session settings.
//
// Author: monsterfei
// Date: 2026-06-30
type AuthConfig struct {
	SessionTTLHours        int  `yaml:"session_ttl_hours"`
	PasswordResetTTLMin    int  `yaml:"password_reset_ttl_min"`
	ExposeResetTokenInDev  bool `yaml:"expose_reset_token_in_dev"`
}
```

Defaults:

```go
if cfg.Auth.SessionTTLHours == 0 {
	cfg.Auth.SessionTTLHours = 168
}
if cfg.Auth.PasswordResetTTLMin == 0 {
	cfg.Auth.PasswordResetTTLMin = 30
}
```

Environment overrides:

```go
overrideInt(&cfg.Auth.SessionTTLHours, "CW_AUTH_SESSION_TTL_HOURS")
overrideInt(&cfg.Auth.PasswordResetTTLMin, "CW_AUTH_PASSWORD_RESET_TTL_MIN")
overrideBool(&cfg.Auth.ExposeResetTokenInDev, "CW_AUTH_EXPOSE_RESET_TOKEN_IN_DEV")
```

Validation:

```go
if c.Auth.SessionTTLHours <= 0 {
	return errors.New("auth.session_ttl_hours must be greater than 0")
}
if c.Auth.PasswordResetTTLMin <= 0 {
	return errors.New("auth.password_reset_ttl_min must be greater than 0")
}
```

Add to `configs/config.example.yaml`:

```yaml
auth:
  session_ttl_hours: 168
  password_reset_ttl_min: 30
  expose_reset_token_in_dev: true
```

- [x] **Step 4: Implement auth API routes**

Create `internal/api/auth.go`:

```go
type AuthService interface {
	Register(context.Context, auth.RegisterRequest) (auth.AuthSession, error)
	Login(context.Context, auth.LoginRequest) (auth.AuthSession, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (model.User, bool, error)
	RequestPasswordReset(context.Context, string) (string, error)
	ConfirmPasswordReset(context.Context, string, string) error
	ChangePassword(context.Context, int64, string, string) error
}

func mountAuthRoutes(mux *http.ServeMux, deps Dependencies)
func currentSessionToken(r *http.Request) string
func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time)
func clearSessionCookie(w http.ResponseWriter)
func requireUser(r *http.Request, deps Dependencies) (model.User, bool, error)
```

Routes:

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
POST /api/v1/auth/password-reset/request
POST /api/v1/auth/password-reset/confirm
POST /api/v1/user/password
```

- [x] **Step 5: Wire auth dependency**

Modify `internal/api/router.go`:

```go
Auth AuthService
```

Call:

```go
mountAuthRoutes(mux, deps)
```

Modify `cmd/server/main.go` to build:

```go
authService := auth.NewService(
	repos.Users,
	repos.Sessions,
	repos.PasswordResetTokens,
	auth.Config{
		SessionTTL:       time.Duration(cfg.Auth.SessionTTLHours) * time.Hour,
		PasswordResetTTL: time.Duration(cfg.Auth.PasswordResetTTLMin) * time.Minute,
		ExposeResetToken: cfg.App.Env != "prod" && cfg.Auth.ExposeResetTokenInDev,
	},
)
```

Pass `Auth: authService` into `api.Dependencies`.

- [x] **Step 6: Run Auth API tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/gofmt -w internal/api/*.go internal/config/config.go cmd/server/main.go && /usr/local/go/bin/go test ./internal/api ./internal/config -run \"Auth|Password\" -v'
```

Expected: PASS.

## Task 5: Session-Protect User APIs And Enforce Plan Limits

**Files:**
- Modify: `internal/api/user.go`
- Modify: `internal/api/user_test.go`
- Modify: `internal/rule/service.go`
- Modify: `internal/storage/alert_rule_repo.go`
- Modify: `internal/user/service.go`
- Modify: `internal/user/service_test.go`

- [x] **Step 1: Write failing session user API tests**

Update `internal/api/user_test.go`:

```go
func TestUserProfileRequiresSession(t *testing.T)
func TestUserProfileUsesSessionUser(t *testing.T)
func TestUserRulesListUsesSessionUser(t *testing.T)
func TestUserRulesPostUsesSessionUser(t *testing.T)
func TestUserAlertsListUsesSessionUser(t *testing.T)
```

Expected behavior:

- No `cw_session` cookie returns `401`.
- Query `user_id` is ignored for `/api/v1/user/*`.
- Rule writes set `rule.UserID` from session user.
- Alert history is read for session user.

- [x] **Step 2: Run session user API tests and verify RED**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run \"User(Profile|Rules|Alerts)\" -v'
```

Expected: FAIL because user APIs still authorize with Bearer Token and parse `user_id`.

- [x] **Step 3: Add rule-count repository support**

Add to `internal/storage/alert_rule_repo.go`:

```go
// CountUserRules returns how many user-scoped rules belong to one user.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param userID User id to count.
// @returns Number of user-owned rules.
func (r AlertRuleRepo) CountUserRules(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.DB.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM alert_rules
		WHERE scope = 'user'
		  AND user_id = $1
	`, userID).Scan(&count)
	return count, err
}
```

Extend rule repository/service interfaces:

```go
CountUserRules(context.Context, int64) (int64, error)
```

- [x] **Step 4: Enforce subscription limits in user service**

Modify `internal/user/service.go` so profile includes:

```go
Plan   string          `json:"plan"`
Status string          `json:"status"`
Limits auth.PlanLimits `json:"limits"`
```

Add:

```go
func (s Service) CanCreateRule(ctx context.Context, user model.User) error
func (s Service) AlertHistoryLimit(user model.User, requested int) int
```

Behavior:

- `free`: max 5 user rules, alert history cap 20.
- `pro`: max 50 user rules, alert history cap 100.
- `vip`: max 200 user rules, alert history cap 200.
- Disabled users receive `403` from user APIs.

- [x] **Step 5: Update user API handlers**

Change `/api/v1/user/*` to call `requireUser(r, deps)` and derive `user.ID`.

For `POST /api/v1/user/rules`:

```go
req.UserID = &currentUser.ID
```

Before `UpsertUserRule`, call the user service limit check. If a free/pro/vip limit is exceeded, return:

```json
{"code":403,"message":"subscription rule limit exceeded","data":null}
```

- [x] **Step 6: Run user API and service tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/gofmt -w internal/api/user.go internal/api/user_test.go internal/user/*.go internal/rule/*.go internal/storage/alert_rule_repo.go && /usr/local/go/bin/go test ./internal/api ./internal/user ./internal/rule ./internal/storage -run \"User|Plan|Rule\" -v'
```

Expected: PASS.

## Task 6: Dashboard Auth UI Migration

**Files:**
- Modify: `internal/api/dashboardui/index.html`
- Modify: `internal/api/dashboardui/app.js`
- Modify: `internal/api/dashboardui/styles.css`
- Modify: `internal/api/user_test.go`

- [x] **Step 1: Write failing Dashboard auth-shell test**

Update `TestDashboardPageIsServed` in `internal/api/user_test.go` to require:

```go
for _, expected := range []string{
	"CryptoWatchtower Dashboard",
	`id="login-email"`,
	`id="login-password"`,
	`id="register-email"`,
	`id="register-password"`,
	`id="logout-button"`,
	"Subscription",
	"Personal Rules",
	"Alert History",
} {
	if !strings.Contains(body, expected) {
		t.Fatalf("expected %q in dashboard page, got %s", expected, body)
	}
}
```

Also assert the old controls are gone:

```go
for _, removed := range []string{`id="bearer-token"`, `id="user-id"`} {
	if strings.Contains(body, removed) {
		t.Fatalf("did not expect legacy control %q in dashboard page", removed)
	}
}
```

- [x] **Step 2: Run Dashboard test and verify RED**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run Dashboard -v'
```

Expected: FAIL because the current Dashboard still has Bearer/User ID controls.

- [x] **Step 3: Update Dashboard HTML**

Modify `internal/api/dashboardui/index.html`:

- Add login form with `login-email` and `login-password`.
- Add register form with `register-email` and `register-password`.
- Add password change form with `current-password` and `new-password`.
- Add `logout-button`.
- Add profile area showing `Subscription`, `plan`, and `limits`.
- Remove `bearer-token` and `user-id` inputs.

- [x] **Step 4: Update Dashboard JavaScript**

Modify `internal/api/dashboardui/app.js`:

- Use `fetch(url, { credentials: "same-origin" })` for all auth and user requests.
- Register: `POST /api/v1/auth/register`.
- Login: `POST /api/v1/auth/login`.
- Logout: `POST /api/v1/auth/logout`.
- Change password: `POST /api/v1/user/password`.
- Load profile/rules/alerts without `user_id`.
- Save rules without `user_id`; backend derives owner from session.
- Never store password, raw session token, or Bearer Token in `localStorage`.

- [x] **Step 5: Check Dashboard JavaScript syntax**

Run:

```bash
node --check internal/api/dashboardui/app.js
```

Expected: PASS.

- [x] **Step 6: Run Dashboard route test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run Dashboard -v'
```

Expected: PASS.

## Task 7: Docs And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`
- Modify: `docs/superpowers/plans/[8]2026-06-30-saas-auth-subscription.md`

- [x] **Step 1: Document auth and subscription usage**

Update docs with:

- Register/Login/Logout curl examples.
- Strong password rule: at least 8 chars, uppercase, lowercase, digit, special character.
- Password reset request/confirm flow.
- Password change flow.
- Session cookie behavior.
- Free/Pro/VIP local entitlement limits.
- Operator APIs remain Bearer Token protected.
- Payment billing is not included in this plan.

- [x] **Step 2: Update master plan status**

In `docs/plan/币圈异动监控平台总体开发计划.md`, keep `SaaS 权限与订阅` unchecked until verification passes during implementation. Add the active plan pointer:

```markdown
当前执行计划：

- `docs/superpowers/plans/[8]2026-06-30-saas-auth-subscription.md`
```

- [x] **Step 3: Run verification gate**

Run:

```bash
node --check internal/api/dashboardui/app.js
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
curl -fsS http://127.0.0.1:18080/dashboard
git diff --check
```

Expected: PASS.

- [x] **Step 4: Mark this plan complete after verification**

After every command in Step 3 passes, update all acceptance criteria and task checkboxes in this plan to `[x]`, then add an execution note with the exact verification evidence.

## Execution Notes

- 2026-06-30: Plan created after `[7]2026-06-30-user-dashboard.md` completed. Scope is auth/session/subscription foundation only; real payment provider billing is deliberately outside this plan.
- 2026-07-01: Implementation completed. Verification evidence:
  - `node --check internal/api/dashboardui/app.js` passed.
  - `docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'` passed after one unrelated collector keepalive timeout was reproduced as non-deterministic and passed on focused/full rerun.
  - `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` passed.
  - `curl -fsS http://127.0.0.1:18080/dashboard` returned the Dashboard HTML with auth controls.
  - `docker run --rm --network host -v "$PWD":/workspace -w /workspace -e CW_INTEGRATION_TESTS=1 -e CW_POSTGRES_DSN='postgres://postgres:CryptoWatchtower_Local_2026!@127.0.0.1:5432/crypto_watchtower?sslmode=disable' golang:1.24 sh -c '/usr/local/go/bin/go test -tags integration ./internal/integration -run AuthRepositories -v'` passed.
  - `git diff --check` passed.
