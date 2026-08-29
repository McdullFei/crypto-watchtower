package api

import (
	"bytes"
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

//go:embed adminui/*
var adminAssets embed.FS

type AdminOverview struct {
	RuleCount         int64      `json:"rule_count"`
	AlertCount24h     int64      `json:"alert_count_24h"`
	EventCount24h     int64      `json:"event_count_24h"`
	NotificationCount int64      `json:"notification_count"`
	LastAlertAt       *time.Time `json:"last_alert_at"`
}

// AdminTrends contains lightweight 24-hour operational trend counters.
//
// Author: monsterfei
// Date: 2026-06-29
type AdminTrends struct {
	Alerts24h        int64                  `json:"alerts_24h"`
	Notifications24h AdminNotificationTrend `json:"notifications_24h"`
	SymbolAlerts24h  []AdminSymbolCount     `json:"symbol_alerts_24h"`
}

// AdminNotificationTrend counts notification outcomes for the trend summary.
//
// Author: monsterfei
// Date: 2026-06-29
type AdminNotificationTrend struct {
	Sent   int64 `json:"sent"`
	Failed int64 `json:"failed"`
}

// AdminSymbolCount contains a compact symbol-to-count aggregate.
//
// Author: monsterfei
// Date: 2026-06-29
type AdminSymbolCount struct {
	Symbol string `json:"symbol"`
	Count  int64  `json:"count"`
}

// AdminListFilter carries bounded Admin list filters.
//
// Author: monsterfei
// Date: 2026-06-30
// modified by monsterfei on 2026-06-30
type AdminListFilter struct {
	Exchange  string
	Symbol    string
	RuleType  string
	EventType string
	Status    string
	Scope     string
	UserID    *int64
	Limit     int
}

type AdminService interface {
	Overview(ctx context.Context) (AdminOverview, error)
	Trends(ctx context.Context) (AdminTrends, error)
	ListRules(ctx context.Context, filter AdminListFilter) ([]model.AlertRule, error)
	ListAlerts(ctx context.Context, filter AdminListFilter) ([]model.Alert, error)
	ListEvents(ctx context.Context, filter AdminListFilter) ([]model.MarketEvent, error)
	ListNotifications(ctx context.Context, filter AdminListFilter) ([]model.NotificationLog, error)
}

// mountAdminRoutes attaches the operator console static files and protected Admin APIs.
//
// Author: monsterfei
// Date: 2026-06-29
// modified by monsterfei on 2026-06-29
// modified by monsterfei on 2026-07-02
func mountAdminRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.Handle("/admin", adminIndexHandler())
	mux.Handle("/admin/", adminFileHandler())
	mux.HandleFunc("/api/v1/admin/overview", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRequest(w, r, deps, func(admin AdminService, w http.ResponseWriter, r *http.Request) {
			data, err := admin.Overview(r.Context())
			if err != nil {
				writeInternalError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": data})
		})
	})
	mux.HandleFunc("/api/v1/admin/trends", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRequest(w, r, deps, func(admin AdminService, w http.ResponseWriter, r *http.Request) {
			data, err := admin.Trends(r.Context())
			if err != nil {
				writeInternalError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": data})
		})
	})
	mux.HandleFunc("/api/v1/admin/rules", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRequest(w, r, deps, func(admin AdminService, w http.ResponseWriter, r *http.Request) {
			data, err := admin.ListRules(r.Context(), adminFilterFromRequest(r))
			if err != nil {
				writeInternalError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": nonNilSlice(data)})
		})
	})
	mux.HandleFunc("/api/v1/admin/alerts", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRequest(w, r, deps, func(admin AdminService, w http.ResponseWriter, r *http.Request) {
			data, err := admin.ListAlerts(r.Context(), adminFilterFromRequest(r))
			if err != nil {
				writeInternalError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": nonNilSlice(data)})
		})
	})
	mux.HandleFunc("/api/v1/admin/events", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRequest(w, r, deps, func(admin AdminService, w http.ResponseWriter, r *http.Request) {
			data, err := admin.ListEvents(r.Context(), adminFilterFromRequest(r))
			if err != nil {
				writeInternalError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": nonNilSlice(data)})
		})
	})
	mux.HandleFunc("/api/v1/admin/notifications", func(w http.ResponseWriter, r *http.Request) {
		handleAdminRequest(w, r, deps, func(admin AdminService, w http.ResponseWriter, r *http.Request) {
			data, err := admin.ListNotifications(r.Context(), adminFilterFromRequest(r))
			if err != nil {
				writeInternalError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok", "data": nonNilSlice(data)})
		})
	})
}

// nonNilSlice returns an empty slice when a list result is nil so JSON responses encode as [].
//
// Author: monsterfei
// Date: 2026-07-02
// @param items List items returned by a service.
// @returns A non-nil list for JSON encoding.
func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func handleAdminRequest(w http.ResponseWriter, r *http.Request, deps Dependencies, fn func(AdminService, http.ResponseWriter, *http.Request)) {
	if !authorize(r, deps.APIBearerToken) {
		writeUnauthorized(w)
		return
	}
	if deps.Admin == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"code":    501,
			"message": "admin service is not configured",
			"data":    nil,
		})
		return
	}
	fn(deps.Admin, w, r)
}

// adminFilterFromRequest parses shared Admin list filters from query parameters.
//
// Author: monsterfei
// Date: 2026-06-30
// @param r HTTP request.
// @returns Admin list filter.
// modified by monsterfei on 2026-06-30
func adminFilterFromRequest(r *http.Request) AdminListFilter {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	var userID *int64
	if raw := r.URL.Query().Get("user_id"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			userID = &parsed
		}
	}
	return AdminListFilter{
		Exchange:  r.URL.Query().Get("exchange"),
		Symbol:    r.URL.Query().Get("symbol"),
		RuleType:  r.URL.Query().Get("rule_type"),
		EventType: r.URL.Query().Get("event_type"),
		Status:    r.URL.Query().Get("status"),
		Scope:     r.URL.Query().Get("scope"),
		UserID:    userID,
		Limit:     limit,
	}
}

func adminIndexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, err := adminAssets.ReadFile("adminui/index.html")
		if err != nil {
			http.Error(w, "admin page not found", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
	})
}

func adminFileHandler() http.Handler {
	sub, _ := fs.Sub(adminAssets, "adminui")
	return http.StripPrefix("/admin/", http.FileServer(http.FS(sub)))
}

// writeInternalError logs an internal failure and returns a safe client response.
//
// Author: monsterfei
// Date: 2026-08-29
// @param w HTTP response writer.
// @param err Internal error retained in server logs.
func writeInternalError(w http.ResponseWriter, err error) {
	slog.Error("api request failed", "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"code":    500,
		"message": "internal server error",
		"data":    nil,
	})
}
