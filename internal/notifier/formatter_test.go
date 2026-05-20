package notifier

import (
	"strings"
	"testing"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

func TestFormatAlertIncludesSymbolAndMessage(t *testing.T) {
	alert := model.Alert{
		Symbol:  "BTCUSDT",
		Title:   "BTCUSDT large aggressive flow",
		Message: "成交额: 150000 USDT",
	}

	out := FormatAlert(alert)
	if !strings.Contains(out, "BTCUSDT") {
		t.Fatal("expected symbol in formatted message")
	}
	if !strings.Contains(out, "150000") {
		t.Fatal("expected message in formatted output")
	}
}
