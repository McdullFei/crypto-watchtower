package api

import (
	"context"
	"net/http"
	"time"
)

type CollectorStatus struct {
	Name          string     `json:"name"`
	Connected     bool       `json:"connected"`
	Reconnects    int64      `json:"reconnects"`
	LastEventAt   *time.Time `json:"last_event_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	Subscribed    []string   `json:"subscribed"`
	LastConnectAt *time.Time `json:"last_connect_at,omitempty"`
}

type CollectorStatusProvider interface {
	Status() CollectorStatus
}

type DependencyStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DependencyStatusProvider interface {
	Name() string
	Check(context.Context) error
}

func NewHealthHandler(collectors []CollectorStatusProvider, dependencyGroups ...[]DependencyStatusProvider) http.Handler {
	var dependencies []DependencyStatusProvider
	if len(dependencyGroups) > 0 {
		dependencies = dependencyGroups[0]
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statuses := make([]CollectorStatus, 0, len(collectors))
		for _, collector := range collectors {
			statuses = append(statuses, collector.Status())
		}

		dependencyStatuses := make(map[string]DependencyStatus, len(dependencies))
		for _, dependency := range dependencies {
			status := DependencyStatus{Status: "ok"}
			if err := dependency.Check(r.Context()); err != nil {
				status.Status = "error"
				status.Error = err.Error()
			}
			dependencyStatuses[dependency.Name()] = status
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"status":       "up",
				"collectors":   statuses,
				"dependencies": dependencyStatuses,
			},
		})
	})
}
