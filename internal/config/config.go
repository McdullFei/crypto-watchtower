package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Env string `yaml:"env"`
	} `yaml:"app"`
	HTTP     HTTPConfig    `yaml:"http"`
	Binance  BinanceConfig `yaml:"binance"`
	OKX      OKXConfig     `yaml:"okx"`
	Webhook  WebhookConfig `yaml:"webhook"`
	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	Telegram struct {
		Enabled       bool   `yaml:"enabled"`
		BotToken      string `yaml:"bot_token"`
		DefaultChatID string `yaml:"default_chat_id"`
		ParseMode     string `yaml:"parse_mode"`
		Mode          string `yaml:"mode"`
	} `yaml:"telegram"`
	API struct {
		BearerToken string `yaml:"bearer_token"`
	} `yaml:"api"`
	Rules     RulesConfig `yaml:"rules"`
	Scheduler struct {
		FundingIntervalSec int `yaml:"funding_interval_sec"`
	} `yaml:"scheduler"`
}

type HTTPConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type RulesConfig struct {
	LargeTradeSingleUSDT float64 `yaml:"large_trade_single_usdt"`
	LargeTradeWindowUSDT float64 `yaml:"large_trade_window_usdt"`
	LiquidationUSDT      float64 `yaml:"liquidation_usdt"`
	FundingAbsPercent    float64 `yaml:"funding_abs_percent"`
}

// BinanceConfig contains optional Binance market-data settings.
//
// Author: __AUTHOR__
// Date: 2026-06-29
type BinanceConfig struct {
	Enabled            bool     `yaml:"enabled"`
	SpotWSBaseURL      string   `yaml:"spot_ws_base_url"`
	FuturesWSBaseURL   string   `yaml:"futures_ws_base_url"`
	FuturesRESTBaseURL string   `yaml:"futures_rest_base_url"`
	Symbols            []string `yaml:"symbols"`
}

// OKXConfig contains optional OKX public market-data settings.
//
// Author: monsterfei
// Date: 2026-06-29
type OKXConfig struct {
	Enabled         bool     `yaml:"enabled"`
	PublicWSBaseURL string   `yaml:"public_ws_base_url"`
	RestBaseURL     string   `yaml:"rest_base_url"`
	Symbols         []string `yaml:"symbols"`
}

// WebhookConfig contains optional generic webhook notification settings.
//
// Author: __AUTHOR__
// Date: 2026-06-29
type WebhookConfig struct {
	Enabled    bool   `yaml:"enabled"`
	URL        string `yaml:"url"`
	Channel    string `yaml:"channel"`
	TimeoutSec int    `yaml:"timeout_sec"`
}

func (h HTTPConfig) Address() string {
	host := h.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := h.Port
	if port == 0 {
		port = 8080
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func Load(path string) (Config, error) {
	var cfg Config
	cfg.Binance.Enabled = true
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	applyEnvOverrides(&cfg)
	applyDefaults(&cfg)
	return cfg, cfg.Validate()
}

func applyDefaults(cfg *Config) {
	if cfg.Telegram.ParseMode == "" {
		cfg.Telegram.ParseMode = "Markdown"
	}
	if cfg.Telegram.Mode == "" {
		cfg.Telegram.Mode = "polling"
	}
	if cfg.Scheduler.FundingIntervalSec == 0 {
		cfg.Scheduler.FundingIntervalSec = 900
	}
	if cfg.Rules.LargeTradeSingleUSDT == 0 {
		cfg.Rules.LargeTradeSingleUSDT = 100000
	}
	if cfg.Rules.LargeTradeWindowUSDT == 0 {
		cfg.Rules.LargeTradeWindowUSDT = 500000
	}
	if cfg.Rules.LiquidationUSDT == 0 {
		cfg.Rules.LiquidationUSDT = 100000
	}
	if cfg.Rules.FundingAbsPercent == 0 {
		cfg.Rules.FundingAbsPercent = 0.08
	}
	if cfg.OKX.PublicWSBaseURL == "" {
		cfg.OKX.PublicWSBaseURL = "wss://ws.okx.com:8443/ws/v5/public"
	}
	if cfg.OKX.RestBaseURL == "" {
		cfg.OKX.RestBaseURL = "https://www.okx.com"
	}
	if cfg.Webhook.Channel == "" {
		cfg.Webhook.Channel = "webhook"
	}
	if cfg.Webhook.TimeoutSec == 0 {
		cfg.Webhook.TimeoutSec = 10
	}
}

func applyEnvOverrides(cfg *Config) {
	overrideString(&cfg.Telegram.BotToken, "CW_TELEGRAM_BOT_TOKEN")
	overrideString(&cfg.Telegram.DefaultChatID, "CW_TELEGRAM_DEFAULT_CHAT_ID")
	overrideString(&cfg.Postgres.DSN, "CW_POSTGRES_DSN")
	overrideString(&cfg.Redis.Addr, "CW_REDIS_ADDR")
	overrideString(&cfg.Redis.Password, "CW_REDIS_PASSWORD")
	overrideString(&cfg.API.BearerToken, "CW_API_BEARER_TOKEN")
	overrideString(&cfg.Binance.SpotWSBaseURL, "CW_BINANCE_SPOT_WS_BASE_URL")
	overrideString(&cfg.Binance.FuturesWSBaseURL, "CW_BINANCE_FUTURES_WS_BASE_URL")
	overrideString(&cfg.Binance.FuturesRESTBaseURL, "CW_BINANCE_FUTURES_REST_BASE_URL")
	overrideBool(&cfg.Binance.Enabled, "CW_BINANCE_ENABLED")
	overrideString(&cfg.OKX.PublicWSBaseURL, "CW_OKX_PUBLIC_WS_BASE_URL")
	overrideString(&cfg.OKX.RestBaseURL, "CW_OKX_REST_BASE_URL")
	overrideBool(&cfg.OKX.Enabled, "CW_OKX_ENABLED")
	overrideString(&cfg.Webhook.URL, "CW_WEBHOOK_URL")
	overrideString(&cfg.Webhook.Channel, "CW_WEBHOOK_CHANNEL")
	overrideBool(&cfg.Webhook.Enabled, "CW_WEBHOOK_ENABLED")
	if value, ok := os.LookupEnv("CW_BINANCE_SYMBOLS"); ok && value != "" {
		cfg.Binance.Symbols = strings.Split(value, ",")
	}
	if value, ok := os.LookupEnv("CW_OKX_SYMBOLS"); ok && value != "" {
		cfg.OKX.Symbols = strings.Split(value, ",")
	}
	if value, ok := os.LookupEnv("CW_REDIS_DB"); ok && value != "" {
		if db, err := strconv.Atoi(value); err == nil {
			cfg.Redis.DB = db
		}
	}
	if value, ok := os.LookupEnv("CW_WEBHOOK_TIMEOUT_SEC"); ok && value != "" {
		if timeoutSec, err := strconv.Atoi(value); err == nil {
			cfg.Webhook.TimeoutSec = timeoutSec
		}
	}
}

func overrideString(target *string, key string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = value
	}
}

// overrideBool replaces target when key contains a parseable boolean value.
//
// Author: monsterfei
// Date: 2026-06-29
func overrideBool(target *bool, key string) {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(value); err == nil {
			*target = parsed
		}
	}
}

func (c Config) Validate() error {
	if c.Binance.Enabled {
		if c.Binance.SpotWSBaseURL == "" {
			return errors.New("binance.spot_ws_base_url is required")
		}
		if c.Binance.FuturesWSBaseURL == "" {
			return errors.New("binance.futures_ws_base_url is required")
		}
		if c.Binance.FuturesRESTBaseURL == "" {
			return errors.New("binance.futures_rest_base_url is required")
		}
		if len(c.Binance.Symbols) == 0 {
			return errors.New("binance.symbols is required")
		}
	}
	if c.Postgres.DSN == "" {
		return errors.New("postgres.dsn is required")
	}
	if c.Redis.Addr == "" {
		return errors.New("redis.addr is required")
	}
	if c.API.BearerToken == "" {
		return errors.New("api.bearer_token is required")
	}
	if c.Telegram.Enabled {
		if c.Telegram.BotToken == "" {
			return errors.New("telegram.bot_token is required when telegram is enabled")
		}
		if c.Telegram.DefaultChatID == "" {
			return errors.New("telegram.default_chat_id is required when telegram is enabled")
		}
	}
	if c.OKX.Enabled {
		if c.OKX.PublicWSBaseURL == "" {
			return errors.New("okx.public_ws_base_url is required when okx is enabled")
		}
		if c.OKX.RestBaseURL == "" {
			return errors.New("okx.rest_base_url is required when okx is enabled")
		}
		if len(c.OKX.Symbols) == 0 {
			return errors.New("okx.symbols is required when okx is enabled")
		}
	}
	if c.Webhook.Enabled && c.Webhook.URL == "" {
		return errors.New("webhook.url is required when webhook is enabled")
	}
	return nil
}
