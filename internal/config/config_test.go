package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRequiresTelegramTokenWhenEnabled(t *testing.T) {
	cfg := Config{}
	cfg.Telegram.Enabled = true
	cfg.API.BearerToken = "token"
	cfg.Postgres.DSN = "postgres://example"
	cfg.Redis.Addr = "localhost:6379"
	cfg.Binance.SpotWSBaseURL = "wss://stream.binance.com:9443/ws"
	cfg.Binance.FuturesWSBaseURL = "wss://fstream.binance.com/ws"
	cfg.Binance.FuturesRESTBaseURL = "https://fapi.binance.com"
	cfg.Binance.Symbols = []string{"BTCUSDT"}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error when telegram is enabled without bot token")
	}
}

func TestLoadAppliesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("" +
		"binance:\n" +
		"  spot_ws_base_url: wss://stream.binance.com:9443/ws\n" +
		"  futures_ws_base_url: wss://fstream.binance.com/ws\n" +
		"  futures_rest_base_url: https://fapi.binance.com\n" +
		"  symbols: [BTCUSDT]\n" +
		"postgres:\n" +
		"  dsn: postgres://from-file\n" +
		"redis:\n" +
		"  addr: localhost:6379\n" +
		"telegram:\n" +
		"  enabled: true\n" +
		"  bot_token: from-file\n" +
		"  default_chat_id: default-chat\n" +
		"api:\n" +
		"  bearer_token: file-token\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CW_TELEGRAM_BOT_TOKEN", "from-env")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Telegram.BotToken != "from-env" {
		t.Fatalf("expected env override, got %q", cfg.Telegram.BotToken)
	}
}

// TestValidateAllowsOKXDisabledWithoutURLs verifies disabled OKX config stays optional.
//
// Author: monsterfei
// Date: 2026-06-29
func TestValidateAllowsOKXDisabledWithoutURLs(t *testing.T) {
	cfg := validConfig()
	cfg.OKX.Enabled = false

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected disabled OKX config to be optional, got %v", err)
	}
}

// TestValidateRequiresOKXFieldsWhenEnabled verifies enabled OKX config is complete.
//
// Author: monsterfei
// Date: 2026-06-29
func TestValidateRequiresOKXFieldsWhenEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  func() Config
	}{
		{
			name: "missing public websocket URL",
			cfg: func() Config {
				cfg := validConfig()
				cfg.OKX.Enabled = true
				cfg.OKX.RestBaseURL = "https://www.okx.com"
				cfg.OKX.Symbols = []string{"BTCUSDT"}
				return cfg
			},
		},
		{
			name: "missing REST URL",
			cfg: func() Config {
				cfg := validConfig()
				cfg.OKX.Enabled = true
				cfg.OKX.PublicWSBaseURL = "wss://ws.okx.com:8443/ws/v5/public"
				cfg.OKX.Symbols = []string{"BTCUSDT"}
				return cfg
			},
		},
		{
			name: "missing symbols",
			cfg: func() Config {
				cfg := validConfig()
				cfg.OKX.Enabled = true
				cfg.OKX.PublicWSBaseURL = "wss://ws.okx.com:8443/ws/v5/public"
				cfg.OKX.RestBaseURL = "https://www.okx.com"
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg().Validate(); err == nil {
				t.Fatalf("expected OKX validation error")
			}
		})
	}
}

// TestLoadAppliesOKXEnvOverrides verifies OKX runtime settings can come from env.
//
// Author: monsterfei
// Date: 2026-06-29
func TestLoadAppliesOKXEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("" +
		"binance:\n" +
		"  spot_ws_base_url: wss://stream.binance.com:9443/ws\n" +
		"  futures_ws_base_url: wss://fstream.binance.com/ws\n" +
		"  futures_rest_base_url: https://fapi.binance.com\n" +
		"  symbols: [BTCUSDT]\n" +
		"okx:\n" +
		"  enabled: false\n" +
		"postgres:\n" +
		"  dsn: postgres://from-file\n" +
		"redis:\n" +
		"  addr: localhost:6379\n" +
		"telegram:\n" +
		"  enabled: false\n" +
		"api:\n" +
		"  bearer_token: file-token\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CW_OKX_ENABLED", "true")
	t.Setenv("CW_OKX_PUBLIC_WS_BASE_URL", "wss://example/ws")
	t.Setenv("CW_OKX_REST_BASE_URL", "https://example.test")
	t.Setenv("CW_OKX_SYMBOLS", "BTCUSDT,ETHUSDT")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.OKX.Enabled {
		t.Fatalf("expected OKX to be enabled from env")
	}
	if cfg.OKX.PublicWSBaseURL != "wss://example/ws" {
		t.Fatalf("expected OKX websocket URL override, got %q", cfg.OKX.PublicWSBaseURL)
	}
	if cfg.OKX.RestBaseURL != "https://example.test" {
		t.Fatalf("expected OKX REST URL override, got %q", cfg.OKX.RestBaseURL)
	}
	if got := cfg.OKX.Symbols; len(got) != 2 || got[0] != "BTCUSDT" || got[1] != "ETHUSDT" {
		t.Fatalf("expected OKX symbols override, got %#v", got)
	}
}

// TestLoadAllowsBinanceDisabledForOKXOnly verifies OKX-only smoke can disable Binance requirements.
//
// Author: monsterfei
// Date: 2026-06-29
func TestLoadAllowsBinanceDisabledForOKXOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("" +
		"okx:\n" +
		"  enabled: true\n" +
		"  public_ws_base_url: wss://example/ws\n" +
		"  rest_base_url: https://example.test\n" +
		"  symbols: [BTCUSDT]\n" +
		"postgres:\n" +
		"  dsn: postgres://from-file\n" +
		"redis:\n" +
		"  addr: localhost:6379\n" +
		"telegram:\n" +
		"  enabled: false\n" +
		"api:\n" +
		"  bearer_token: file-token\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CW_BINANCE_ENABLED", "false")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load OKX-only config: %v", err)
	}
	if cfg.Binance.Enabled {
		t.Fatalf("expected Binance to be disabled from env")
	}
	if !cfg.OKX.Enabled {
		t.Fatalf("expected OKX to remain enabled")
	}
}

// TestLoadAppliesWebhookEnvOverrides verifies Discord/Webhook runtime settings can come from env.
//
// Author: monsterfei
// Date: 2026-06-29
func TestLoadAppliesWebhookEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("" +
		"binance:\n" +
		"  enabled: false\n" +
		"okx:\n" +
		"  enabled: true\n" +
		"  public_ws_base_url: wss://example/ws\n" +
		"  rest_base_url: https://example.test\n" +
		"  symbols: [BTCUSDT]\n" +
		"postgres:\n" +
		"  dsn: postgres://from-file\n" +
		"redis:\n" +
		"  addr: localhost:6379\n" +
		"telegram:\n" +
		"  enabled: false\n" +
		"api:\n" +
		"  bearer_token: file-token\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CW_WEBHOOK_ENABLED", "true")
	t.Setenv("CW_WEBHOOK_URL", "https://discord.example/webhook")
	t.Setenv("CW_WEBHOOK_CHANNEL", "discord")
	t.Setenv("CW_WEBHOOK_TIMEOUT_SEC", "7")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Webhook.Enabled || cfg.Webhook.URL != "https://discord.example/webhook" || cfg.Webhook.Channel != "discord" || cfg.Webhook.TimeoutSec != 7 {
		t.Fatalf("unexpected webhook config: %+v", cfg.Webhook)
	}
}

// TestLoadAppliesSummaryEnvOverrides verifies AI summary runtime settings can come from env.
//
// Author: monsterfei
// Date: 2026-06-30
func TestLoadAppliesSummaryEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("" +
		"binance:\n" +
		"  enabled: false\n" +
		"okx:\n" +
		"  enabled: true\n" +
		"  public_ws_base_url: wss://example/ws\n" +
		"  rest_base_url: https://example.test\n" +
		"  symbols: [BTCUSDT]\n" +
		"postgres:\n" +
		"  dsn: postgres://from-file\n" +
		"redis:\n" +
		"  addr: localhost:6379\n" +
		"telegram:\n" +
		"  enabled: false\n" +
		"api:\n" +
		"  bearer_token: file-token\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CW_SUMMARY_ENABLED", "true")
	t.Setenv("CW_SUMMARY_INTERVAL_SEC", "900")
	t.Setenv("CW_SUMMARY_WINDOW_SEC", "900")
	t.Setenv("CW_SUMMARY_MAX_ITEMS", "40")
	t.Setenv("CW_SUMMARY_PROVIDER", "template")
	t.Setenv("CW_SUMMARY_DISCLAIMER", "不构成投资建议")
	t.Setenv("CW_SUMMARY_API_BASE_URL", "https://api.example.test/v1")
	t.Setenv("CW_SUMMARY_API_KEY", "summary-key")
	t.Setenv("CW_SUMMARY_MODEL", "summary-model")
	t.Setenv("CW_SUMMARY_TIMEOUT_SEC", "8")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Summary.Enabled || cfg.Summary.IntervalSec != 900 || cfg.Summary.WindowSec != 900 || cfg.Summary.MaxItems != 40 || cfg.Summary.Provider != "template" || cfg.Summary.Disclaimer != "不构成投资建议" || cfg.Summary.APIBaseURL != "https://api.example.test/v1" || cfg.Summary.APIKey != "summary-key" || cfg.Summary.Model != "summary-model" || cfg.Summary.TimeoutSec != 8 {
		t.Fatalf("unexpected summary config: %+v", cfg.Summary)
	}
}

// validConfig returns a minimal valid configuration for validation tests.
//
// Author: monsterfei
// Date: 2026-06-29
func validConfig() Config {
	cfg := Config{}
	cfg.API.BearerToken = "token"
	cfg.Postgres.DSN = "postgres://example"
	cfg.Redis.Addr = "localhost:6379"
	cfg.Binance.SpotWSBaseURL = "wss://stream.binance.com:9443/ws"
	cfg.Binance.FuturesWSBaseURL = "wss://fstream.binance.com/ws"
	cfg.Binance.FuturesRESTBaseURL = "https://fapi.binance.com"
	cfg.Binance.Symbols = []string{"BTCUSDT"}
	return cfg
}
