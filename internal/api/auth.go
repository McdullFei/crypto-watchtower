package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	authsvc "github.com/renfei198727/crypto-watchtower/internal/auth"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

const sessionCookieName = "cw_session"

// AuthService defines session-account operations required by API handlers.
//
// Author: monsterfei
// Date: 2026-07-01
type AuthService interface {
	Register(context.Context, authsvc.RegisterRequest) (authsvc.AuthSession, error)
	Login(context.Context, authsvc.LoginRequest) (authsvc.AuthSession, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (model.User, bool, error)
	RequestPasswordReset(context.Context, string) (string, error)
	ConfirmPasswordReset(context.Context, string, string) error
	ChangePassword(context.Context, int64, string, string) error
}

// authEmailPasswordRequest contains register and login credentials.
//
// Author: monsterfei
// Date: 2026-07-01
type authEmailPasswordRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// passwordResetRequest contains a password reset request payload.
//
// Author: monsterfei
// Date: 2026-07-01
type passwordResetRequest struct {
	Email string `json:"email"`
}

// passwordResetConfirmRequest contains a reset-token password update payload.
//
// Author: monsterfei
// Date: 2026-07-01
type passwordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// changePasswordRequest contains a logged-in password change payload.
//
// Author: monsterfei
// Date: 2026-07-01
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// authUserResponse contains safe account data returned by auth APIs.
//
// Author: monsterfei
// Date: 2026-07-01
type authUserResponse struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	Plan      string    `json:"plan"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// mountAuthRoutes attaches public auth routes and logged-in password changes.
//
// Author: monsterfei
// Date: 2026-07-01
// @param mux HTTP multiplexer to receive routes.
// @param deps Runtime dependencies required by auth APIs.
func mountAuthRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/auth/register", methodOnly(http.MethodPost, handleAuthRegister(deps)))
	mux.HandleFunc("/api/v1/auth/login", methodOnly(http.MethodPost, handleAuthLogin(deps)))
	mux.HandleFunc("/api/v1/auth/logout", methodOnly(http.MethodPost, handleAuthLogout(deps)))
	mux.HandleFunc("/api/v1/auth/password-reset/request", methodOnly(http.MethodPost, handlePasswordResetRequest(deps)))
	mux.HandleFunc("/api/v1/auth/password-reset/confirm", methodOnly(http.MethodPost, handlePasswordResetConfirm(deps)))
	mux.HandleFunc("/api/v1/user/password", methodOnly(http.MethodPost, handleChangePassword(deps)))
}

// methodOnly wraps a handler with an HTTP method check.
//
// Author: monsterfei
// Date: 2026-07-01
// @param method Allowed HTTP method.
// @param next Handler to invoke when method matches.
// @returns HTTP handler function with method guard.
func methodOnly(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

// handleAuthRegister registers an account and sets its session cookie.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by auth APIs.
// @returns HTTP handler for registration.
func handleAuthRegister(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuthService(w, deps)
		if !ok {
			return
		}
		var req authEmailPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid json body", "data": nil})
			return
		}
		session, err := auth.Register(r.Context(), authsvc.RegisterRequest{Email: req.Email, Password: req.Password})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": err.Error(), "data": nil})
			return
		}
		setSessionCookie(w, session.Token, session.ExpiresAt)
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "registered", "data": safeAuthUser(session)})
	}
}

// handleAuthLogin validates credentials and sets a session cookie.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by auth APIs.
// @returns HTTP handler for login.
func handleAuthLogin(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuthService(w, deps)
		if !ok {
			return
		}
		var req authEmailPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid json body", "data": nil})
			return
		}
		session, err := auth.Login(r.Context(), authsvc.LoginRequest{Email: req.Email, Password: req.Password})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": err.Error(), "data": nil})
			return
		}
		setSessionCookie(w, session.Token, session.ExpiresAt)
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "logged in", "data": safeAuthUser(session)})
	}
}

// handleAuthLogout revokes and clears the current session cookie.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by auth APIs.
// @returns HTTP handler for logout.
func handleAuthLogout(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuthService(w, deps)
		if !ok {
			return
		}
		if err := auth.Logout(r.Context(), currentSessionToken(r)); err != nil {
			writeInternalError(w, err)
			return
		}
		clearSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "logged out", "data": nil})
	}
}

// handlePasswordResetRequest accepts reset requests without revealing account existence.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by auth APIs.
// @returns HTTP handler for reset requests.
func handlePasswordResetRequest(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuthService(w, deps)
		if !ok {
			return
		}
		var req passwordResetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid json body", "data": nil})
			return
		}
		resetToken, err := auth.RequestPasswordReset(r.Context(), req.Email)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		data := map[string]any{"accepted": true}
		if resetToken != "" {
			data["reset_token"] = resetToken
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "password reset accepted", "data": data})
	}
}

// handlePasswordResetConfirm consumes a reset token and updates the password.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by auth APIs.
// @returns HTTP handler for reset confirmation.
func handlePasswordResetConfirm(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuthService(w, deps)
		if !ok {
			return
		}
		var req passwordResetConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid json body", "data": nil})
			return
		}
		if err := auth.ConfirmPasswordReset(r.Context(), req.Token, req.NewPassword); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": err.Error(), "data": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "password reset", "data": nil})
	}
}

// handleChangePassword changes the current user's password.
//
// Author: monsterfei
// Date: 2026-07-01
// @param deps Runtime dependencies required by auth APIs.
// @returns HTTP handler for logged-in password changes.
// modified by monsterfei on 2026-08-20
func handleChangePassword(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok, err := requireDashboardUser(w, r, deps)
		if err != nil || !ok {
			return
		}
		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid json body", "data": nil})
			return
		}
		if err := deps.Auth.ChangePassword(r.Context(), user.ID, req.CurrentPassword, req.NewPassword); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": err.Error(), "data": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "password changed", "data": nil})
	}
}

// requireAuthService writes a not-configured response when auth is unavailable.
//
// Author: monsterfei
// Date: 2026-07-01
// @param w HTTP response writer.
// @param deps Runtime dependencies.
// @returns Auth service and whether it is configured.
func requireAuthService(w http.ResponseWriter, deps Dependencies) (AuthService, bool) {
	if deps.Auth == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"code": 501, "message": "auth service is not configured", "data": nil})
		return nil, false
	}
	return deps.Auth, true
}

// requireUser resolves the current user from the session cookie.
//
// Author: monsterfei
// Date: 2026-07-01
// @param r HTTP request containing the session cookie.
// @param deps Runtime dependencies.
// @returns Current user, whether it is authenticated, and lookup error.
func requireUser(r *http.Request, deps Dependencies) (model.User, bool, error) {
	if deps.Auth == nil {
		return model.User{}, false, nil
	}
	return deps.Auth.CurrentUser(r.Context(), currentSessionToken(r))
}

// currentSessionToken returns the raw session token from the request cookie.
//
// Author: monsterfei
// Date: 2026-07-01
// @param r HTTP request.
// @returns Raw session token or empty string.
func currentSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// setSessionCookie writes an HttpOnly session cookie.
//
// Author: monsterfei
// Date: 2026-07-01
// @param w HTTP response writer.
// @param token Raw session token.
// @param expiresAt Cookie expiry timestamp.
func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie in the browser.
//
// Author: monsterfei
// Date: 2026-07-01
// @param w HTTP response writer.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// safeAuthUser converts an auth session into a password-free response.
//
// Author: monsterfei
// Date: 2026-07-01
// @param session Auth session returned by the auth service.
// @returns Safe account response.
func safeAuthUser(session authsvc.AuthSession) authUserResponse {
	return authUserResponse{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		Plan:      session.User.Plan,
		Status:    session.User.Status,
		ExpiresAt: session.ExpiresAt,
	}
}
