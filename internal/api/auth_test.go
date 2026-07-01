package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authsvc "github.com/renfei198727/crypto-watchtower/internal/auth"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// stubAuthService captures auth API calls for handler tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
type stubAuthService struct {
	registerErr error
	loginErr    error
	currentUser model.User
	currentOK   bool
	logoutToken string
	resetToken  string
	changedUser int64
}

// Register returns a configured registration response for auth API tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubAuthService) Register(_ context.Context, req authsvc.RegisterRequest) (authsvc.AuthSession, error) {
	if s.registerErr != nil {
		return authsvc.AuthSession{}, s.registerErr
	}
	return authsvc.AuthSession{
		User:      model.User{ID: 42, Email: strings.ToLower(strings.TrimSpace(req.Email)), Plan: model.UserPlanFree, Status: model.UserStatusActive},
		Token:     "register-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}, nil
}

// Login returns a configured login response for auth API tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubAuthService) Login(_ context.Context, req authsvc.LoginRequest) (authsvc.AuthSession, error) {
	if s.loginErr != nil {
		return authsvc.AuthSession{}, s.loginErr
	}
	return authsvc.AuthSession{
		User:      model.User{ID: 42, Email: strings.ToLower(strings.TrimSpace(req.Email)), Plan: model.UserPlanFree, Status: model.UserStatusActive},
		Token:     "login-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}, nil
}

// Logout records the token revoked by the auth API.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubAuthService) Logout(_ context.Context, token string) error {
	s.logoutToken = token
	return nil
}

// CurrentUser resolves the configured current user for auth API tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubAuthService) CurrentUser(_ context.Context, _ string) (model.User, bool, error) {
	return s.currentUser, s.currentOK, nil
}

// RequestPasswordReset returns the configured reset token for auth API tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubAuthService) RequestPasswordReset(_ context.Context, _ string) (string, error) {
	return s.resetToken, nil
}

// ConfirmPasswordReset validates reset confirmation for auth API tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubAuthService) ConfirmPasswordReset(_ context.Context, _ string, password string) error {
	if password == "weak" {
		return errors.New("password must include uppercase, lowercase, digit, and special characters")
	}
	return nil
}

// ChangePassword records the user id used for password changes in auth API tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func (s *stubAuthService) ChangePassword(_ context.Context, userID int64, _ string, _ string) error {
	s.changedUser = userID
	return nil
}

// TestAuthRegisterRejectsWeakPassword verifies registration errors are returned as bad requests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestAuthRegisterRejectsWeakPassword(t *testing.T) {
	auth := &stubAuthService{registerErr: errors.New("password must be at least 8 characters")}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"user@example.com","password":"weak"}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: auth}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAuthRegisterSetsSessionCookie verifies registration sets an HttpOnly session cookie.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestAuthRegisterSetsSessionCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"User@example.com","password":"Strong1!"}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}}).ServeHTTP(rec, req)

	assertSessionCookie(t, rec, "cw_session", "register-token")
}

// TestAuthLoginSetsSessionCookie verifies login sets an HttpOnly session cookie.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestAuthLoginSetsSessionCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"Strong1!"}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}}).ServeHTTP(rec, req)

	assertSessionCookie(t, rec, "cw_session", "login-token")
}

// TestAuthLogoutClearsSessionCookie verifies logout revokes and clears the current session cookie.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestAuthLogoutClearsSessionCookie(t *testing.T) {
	auth := &stubAuthService{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "cw_session", Value: "old-token"})
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: auth}).ServeHTTP(rec, req)

	if auth.logoutToken != "old-token" {
		t.Fatalf("expected logout token, got %q", auth.logoutToken)
	}
	cookie := findCookie(rec.Result().Cookies(), "cw_session")
	if cookie == nil || cookie.Value != "" || cookie.MaxAge != -1 {
		t.Fatalf("expected expired session cookie, got %+v", cookie)
	}
}

// TestPasswordResetRequestReturnsGenericResponse verifies reset requests hide account existence.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestPasswordResetRequestReturnsGenericResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"missing@example.com"}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "missing@example.com") || strings.Contains(rec.Body.String(), "reset_token") {
		t.Fatalf("reset response leaked account or token details: %s", rec.Body.String())
	}
}

// TestPasswordResetConfirmRejectsWeakPassword verifies reset confirmation enforces password policy errors.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestPasswordResetConfirmRejectsWeakPassword(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"token":"reset-token","new_password":"weak"}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestChangePasswordRequiresSession verifies password changes require a valid session.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func TestChangePasswordRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/password", strings.NewReader(`{"current_password":"Strong1!","new_password":"Better1!"}`))
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{Auth: &stubAuthService{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// assertSessionCookie verifies a response sets the expected session cookie attributes.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func assertSessionCookie(t *testing.T, rec *httptest.ResponseRecorder, name string, value string) {
	t.Helper()
	cookie := findCookie(rec.Result().Cookies(), name)
	if cookie == nil {
		t.Fatalf("expected %s cookie in %v", name, rec.Result().Cookies())
	}
	if cookie.Value != value || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}
}

// findCookie returns a cookie by name from a response cookie list.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// decodeAuthPayload decodes a standard auth API response for tests.
//
// Author: __AUTHOR__
// Date: 2026-07-01
func decodeAuthPayload(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) > 0 && string(payload.Data) != "null" {
		if err := json.Unmarshal(payload.Data, target); err != nil {
			t.Fatalf("decode data: %v", err)
		}
	}
}
