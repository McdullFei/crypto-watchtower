package notifier

import (
	"strings"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func FormatAlert(alert model.Alert) string {
	parts := []string{alert.Title}
	if alert.Message != "" {
		parts = append(parts, alert.Message)
	}
	return strings.Join(parts, "\n\n")
}
