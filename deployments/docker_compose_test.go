package deployments_test

import (
	"os"
	"strings"
	"testing"
)

// TestDockerComposePassesBinanceEndpointOverrides verifies compose exposes Binance endpoint overrides.
//
// Author: monsterfei
// Date: 2026-07-02
func TestDockerComposePassesBinanceEndpointOverrides(t *testing.T) {
	raw, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	content := string(raw)
	expected := []string{
		"CW_BINANCE_SPOT_WS_BASE_URL: ${CW_BINANCE_SPOT_WS_BASE_URL:-wss://stream.binance.com:9443/ws}",
		"CW_BINANCE_FUTURES_WS_BASE_URL: ${CW_BINANCE_FUTURES_WS_BASE_URL:-wss://fstream.binance.com/ws}",
		"CW_BINANCE_FUTURES_REST_BASE_URL: ${CW_BINANCE_FUTURES_REST_BASE_URL:-https://fapi.binance.com}",
	}
	for _, item := range expected {
		if !strings.Contains(content, item) {
			t.Fatalf("expected docker-compose.yml to pass through %q", item)
		}
	}
}
