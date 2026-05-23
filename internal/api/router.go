package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type AlertSender interface {
	Send(context.Context, model.Alert) error
}

type Dependencies struct {
	APIBearerToken string
	Symbols        []string
	RuleConfig     config.RulesConfig
	Rules          RuleService
	Admin          AdminService
	Telegram       AlertSender
	Collectors     []CollectorStatusProvider
	Dependencies   []DependencyStatusProvider
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", NewHealthHandler(deps.Collectors, deps.Dependencies))
	mountAdminRoutes(mux, deps)
	mux.HandleFunc("/api/v1/symbols", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data":    deps.Symbols,
		})
	})
	mux.HandleFunc("/api/v1/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleGetRules(deps).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost {
			handlePostRules(deps).ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/alerts/test", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(r, deps.APIBearerToken) {
			writeUnauthorized(w)
			return
		}
		if deps.Telegram != nil {
			_ = deps.Telegram.Send(r.Context(), model.Alert{
				ID:      "test-alert",
				Symbol:  "BTCUSDT",
				Title:   "Alert test",
				Message: "CryptoWatchtower alert pipeline test",
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "test alert accepted",
			"data":    nil,
		})
	})
	mux.HandleFunc("/api/v1/telegram/test", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(r, deps.APIBearerToken) {
			writeUnauthorized(w)
			return
		}
		if deps.Telegram != nil {
			_ = deps.Telegram.Send(r.Context(), model.Alert{
				ID:      "test-telegram-alert",
				Symbol:  "BTCUSDT",
				Title:   "Telegram test",
				Message: "CryptoWatchtower test alert",
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "telegram test accepted",
			"data":    nil,
		})
	})
	return mux
}

func authorize(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	return strings.TrimPrefix(auth, "Bearer ") == token
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]any{
		"code":    401,
		"message": "unauthorized",
		"data":    nil,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
