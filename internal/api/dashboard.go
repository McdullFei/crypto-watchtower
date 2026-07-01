package api

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"time"
)

//go:embed dashboardui/*
var dashboardAssets embed.FS

// mountDashboardRoutes attaches the user dashboard static files.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @param mux HTTP multiplexer to receive dashboard routes.
func mountDashboardRoutes(mux *http.ServeMux) {
	mux.Handle("/dashboard", dashboardIndexHandler())
	mux.Handle("/dashboard/", dashboardFileHandler())
}

// dashboardIndexHandler serves the dashboard HTML shell.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @returns HTTP handler for the dashboard root page.
func dashboardIndexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, err := dashboardAssets.ReadFile("dashboardui/index.html")
		if err != nil {
			http.Error(w, "dashboard page not found", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
	})
}

// dashboardFileHandler serves embedded dashboard assets.
//
// Author: __AUTHOR__
// Date: 2026-06-30
// @returns HTTP handler for dashboard asset paths.
func dashboardFileHandler() http.Handler {
	sub, _ := fs.Sub(dashboardAssets, "dashboardui")
	return http.StripPrefix("/dashboard/", http.FileServer(http.FS(sub)))
}
