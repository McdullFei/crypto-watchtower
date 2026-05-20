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
