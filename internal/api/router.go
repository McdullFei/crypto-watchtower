package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/model"
)

type AlertSender interface {
	Send(context.Context, model.Alert) error
}

// EventHandler handles replayed market events through the normal alert pipeline.
//
// Author: __AUTHOR__
// Date: 2026-07-02
type EventHandler interface {
	HandleEvent(context.Context, model.MarketEvent) error
}

type Dependencies struct {
	APIBearerToken  string
	Symbols         []string
	RuleConfig      config.RulesConfig
	Rules           RuleService
	Admin           AdminService
	User            UserService
	Auth            AuthService
	TelegramBinding TelegramBindingService
	Telegram        AlertSender
	Events          EventHandler
	Collectors      []CollectorStatusProvider
	Dependencies    []DependencyStatusProvider
}

// replayEventRequest carries one protected SIT market event replay payload.
//
// Author: __AUTHOR__
// Date: 2026-07-02
type replayEventRequest struct {
	ID         string         `json:"id"`
	Exchange   string         `json:"exchange"`
	MarketType string         `json:"market_type"`
	Symbol     string         `json:"symbol"`
	EventType  string         `json:"event_type"`
	Side       string         `json:"side"`
	Price      float64        `json:"price"`
	Quantity   float64        `json:"quantity"`
	Notional   float64        `json:"notional"`
	Metadata   map[string]any `json:"metadata"`
	EventTime  time.Time      `json:"event_time"`
}

// NewRouter wires public, user-facing, and operator routes into one HTTP handler.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param deps Runtime dependencies required by API routes.
// @returns HTTP handler for the service.
// modified by __AUTHOR__ on 2026-06-30
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", NewHealthHandler(deps.Collectors, deps.Dependencies))
	mountAdminRoutes(mux, deps)
	mountDashboardRoutes(mux)
	mountAuthRoutes(mux, deps)
	mountUserRoutes(mux, deps)
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
	mux.HandleFunc("/api/v1/admin/replay-event", handleReplayEvent(deps))
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

// handleReplayEvent accepts one protected SIT event and sends it through the configured event pipeline.
//
// Author: __AUTHOR__
// Date: 2026-07-02
// @param deps Runtime dependencies including the event handler.
// @returns HTTP handler for market event replay.
func handleReplayEvent(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !authorize(r, deps.APIBearerToken) {
			writeUnauthorized(w)
			return
		}
		if deps.Events == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"code":    501,
				"message": "event handler is not configured",
				"data":    nil,
			})
			return
		}
		var req replayEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":    400,
				"message": "invalid json body",
				"data":    nil,
			})
			return
		}
		event, err := req.toModel(time.Now().UTC())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":    400,
				"message": err.Error(),
				"data":    nil,
			})
			return
		}
		if err := deps.Events.HandleEvent(r.Context(), event); err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "event replayed",
			"data":    event,
		})
	}
}

// toModel validates and converts a replay request into a market event.
//
// Author: __AUTHOR__
// Date: 2026-07-02
// @param now Current time used for default timestamps.
// @returns Market event model or validation error.
func (r replayEventRequest) toModel(now time.Time) (model.MarketEvent, error) {
	if r.Exchange == "" || r.Symbol == "" || r.EventType == "" {
		return model.MarketEvent{}, badRequestError("exchange, symbol, and event_type are required")
	}
	if r.ID == "" {
		r.ID = "replay-" + r.Exchange + "-" + r.Symbol + "-" + r.EventType + "-" + now.Format("20060102150405.000000000")
	}
	if r.MarketType == "" {
		r.MarketType = "spot"
	}
	if r.EventTime.IsZero() {
		r.EventTime = now
	}
	return model.MarketEvent{
		ID:         r.ID,
		Exchange:   r.Exchange,
		MarketType: r.MarketType,
		Symbol:     r.Symbol,
		EventType:  r.EventType,
		Side:       r.Side,
		Price:      r.Price,
		Quantity:   r.Quantity,
		Notional:   r.Notional,
		Metadata:   r.Metadata,
		EventTime:  r.EventTime,
		CreatedAt:  now,
	}, nil
}

// badRequestError is a minimal validation error for replay request parsing.
//
// Author: __AUTHOR__
// Date: 2026-07-02
type badRequestError string

// Error returns the validation failure message.
//
// Author: __AUTHOR__
// Date: 2026-07-02
// @returns Validation error text.
func (e badRequestError) Error() string {
	return string(e)
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
