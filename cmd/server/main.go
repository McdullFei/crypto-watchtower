package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/admin"
	"github.com/renfei198727/crypto-watchtower/internal/api"
	authsvc "github.com/renfei198727/crypto-watchtower/internal/auth"
	"github.com/renfei198727/crypto-watchtower/internal/collector"
	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/notifier"
	"github.com/renfei198727/crypto-watchtower/internal/rule"
	"github.com/renfei198727/crypto-watchtower/internal/scheduler"
	"github.com/renfei198727/crypto-watchtower/internal/storage"
	"github.com/renfei198727/crypto-watchtower/internal/summary"
	"github.com/renfei198727/crypto-watchtower/internal/user"
)

func main() {
	cfgPath := "configs/config.example.yaml"
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		cfgPath = env
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	postgres, err := storage.NewPostgres(ctx, cfg.Postgres.DSN)
	if err != nil {
		slog.Error("init postgres", "err", err)
		os.Exit(1)
	}
	defer postgres.Close()

	redisClient := storage.NewRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer func() { _ = redisClient.Close() }()

	migrationRunner, err := storage.NewFileMigrationRunner(storage.NewPostgresMigrationDB(postgres), "migrations")
	if err != nil {
		slog.Error("load migrations", "err", err)
		os.Exit(1)
	}
	if err := migrationRunner.Run(ctx); err != nil {
		slog.Error("run migrations", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations ready")

	bus := eventbus.New(256)
	repos := storage.NewRepositories(postgres)
	tg := notifier.NewTelegramNotifier(cfg.Telegram.BotToken, cfg.Telegram.DefaultChatID, nil)
	engine := rule.NewEngine(rule.Config{
		LargeTradeThreshold:       cfg.Rules.LargeTradeSingleUSDT,
		LargeTradeWindowThreshold: cfg.Rules.LargeTradeWindowUSDT,
		LargeTradeWindowSec:       60,
		LiquidationThreshold:      cfg.Rules.LiquidationUSDT,
		FundingAbsThreshold:       cfg.Rules.FundingAbsPercent,
	})
	ruleService := rule.NewRuntimeRuleService(repos.AlertRules, engine)
	if err := ruleService.Load(ctx); err != nil {
		slog.Error("load runtime rules", "err", err)
		os.Exit(1)
	}
	adminService := admin.NewService(repos)
	userService := user.NewService(repos.Users, repos.Alerts, repos.AlertRules, repos.NotificationLogs)
	telegramBindingService := user.NewTelegramBindingService(repos.TelegramBindingTokens, repos.Users, user.TelegramBindingConfig{})
	authService := authsvc.NewService(
		repos.Users,
		repos.Sessions,
		repos.PasswordResetTokens,
		authsvc.Config{
			SessionTTL:       time.Duration(cfg.Auth.SessionTTLHours) * time.Hour,
			PasswordResetTTL: time.Duration(cfg.Auth.PasswordResetTTLMin) * time.Minute,
			ExposeResetToken: cfg.App.Env != "prod" && cfg.Auth.ExposeResetTokenInDev,
		},
	)
	pipeline := rule.NewPipeline(engine, repos, redisClient, buildNotificationSenders(cfg, tg)...).
		WithUserFanout(repos.AlertRules, "telegram", tg)

	go func() {
		sub := bus.Subscribe(ctx)
		for event := range sub {
			if err := pipeline.HandleEvent(ctx, event); err != nil {
				slog.Error("handle event", "err", err, "event_id", event.ID)
			}
		}
	}()

	okxInstruments := collector.NewOKXInstrumentStore(nil)
	if cfg.OKX.Enabled {
		fetcher := collector.NewOKXInstrumentFetcher(cfg.OKX.RestBaseURL, nil)
		spotInstruments, err := fetcher.Fetch(ctx, "SPOT")
		if err != nil {
			slog.Error("load okx spot instruments", "err", err)
			os.Exit(1)
		}
		swapInstruments, err := fetcher.Fetch(ctx, "SWAP")
		if err != nil {
			slog.Error("load okx swap instruments", "err", err)
			os.Exit(1)
		}
		okxInstruments = collector.NewOKXInstrumentStore(append(spotInstruments, swapInstruments...))
	}

	marketCollectors, err := buildMarketCollectors(cfg, bus, okxInstruments)
	if err != nil {
		slog.Error("build market collectors", "err", err)
		os.Exit(1)
	}
	for _, marketCollector := range marketCollectors {
		go func(marketCollector runtimeCollector) {
			if err := marketCollector.Start(ctx); err != nil && ctx.Err() == nil {
				slog.Error("collector stopped", "name", marketCollector.Name(), "err", err)
			}
		}(marketCollector)
	}

	if cfg.Binance.Enabled {
		fundingFetcher := collector.NewFundingFetcher(cfg.Binance.FuturesRESTBaseURL, cfg.Binance.Symbols, bus)
		fundingJob := scheduler.NewFundingJob(fundingFetcher, time.Duration(cfg.Scheduler.FundingIntervalSec)*time.Second)
		go fundingJob.Start(ctx)
	}

	if cfg.Summary.Enabled {
		summaryService := buildSummaryService(cfg, repos)
		summaryJob := scheduler.NewSummaryJob(summaryService, time.Duration(cfg.Summary.IntervalSec)*time.Second)
		go summaryJob.Start(ctx)
	}

	if cfg.Telegram.Enabled && cfg.Telegram.Mode == "polling" {
		poller := notifier.NewTelegramPoller(
			notifier.NewTelegramClient(cfg.Telegram.BotToken, "", nil),
			repos.Users,
			ruleService,
			telegramBindingService,
			notifier.TelegramPollerConfig{
				StatusText: "CryptoWatchtower is running.",
				TestAlert: model.Alert{
					Symbol:  "BTCUSDT",
					Title:   "Telegram test",
					Message: "CryptoWatchtower test alert",
				},
			},
		)
		go func() {
			if err := poller.Start(ctx); err != nil && ctx.Err() == nil {
				slog.Error("telegram poller stopped", "err", err)
			}
		}()
	}

	router := api.NewRouter(api.Dependencies{
		APIBearerToken:  cfg.API.BearerToken,
		Symbols:         runtimeSymbols(cfg),
		RuleConfig:      cfg.Rules,
		Rules:           ruleService,
		Admin:           adminService,
		User:            userService,
		Auth:            authService,
		TelegramBinding: telegramBindingService,
		Telegram:        tg,
		Collectors:      collectorHealthAdapters(marketCollectors),
		Dependencies: []api.DependencyStatusProvider{
			dependencyHealthAdapter{name: "postgres", check: postgres.Ping},
			dependencyHealthAdapter{name: "redis", check: func(ctx context.Context) error {
				return redisClient.Ping(ctx).Err()
			}},
		},
	})

	server := &http.Server{
		Addr:              cfg.HTTP.Address(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("http server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown", "err", err)
	}
}

// runtimeCollector is the common runtime surface for exchange collectors.
//
// Author: monsterfei
// Date: 2026-06-29
type runtimeCollector interface {
	Name() string
	Start(context.Context) error
	Subscribe([]string) error
	Status() collector.Status
}

// buildMarketCollectors creates subscribed market collectors from runtime config.
//
// Author: monsterfei
// Date: 2026-06-29
func buildMarketCollectors(cfg config.Config, bus *eventbus.Bus, okxInstruments collector.OKXInstrumentStore) ([]runtimeCollector, error) {
	collectors := make([]runtimeCollector, 0, 3)
	if cfg.Binance.Enabled {
		spotCollector := collector.NewBinanceWSCollector(collector.MarketTypeSpot, cfg.Binance.SpotWSBaseURL, bus)
		futuresCollector := collector.NewBinanceWSCollector(collector.MarketTypeFutures, cfg.Binance.FuturesWSBaseURL, bus)
		if err := spotCollector.Subscribe(cfg.Binance.Symbols); err != nil {
			return nil, err
		}
		if err := futuresCollector.Subscribe(cfg.Binance.Symbols); err != nil {
			return nil, err
		}
		collectors = append(collectors, spotCollector, futuresCollector)
	}

	if cfg.OKX.Enabled {
		okxCollector := collector.NewOKXWSCollector(cfg.OKX.PublicWSBaseURL, bus, okxInstruments)
		if err := okxCollector.Subscribe(cfg.OKX.Symbols); err != nil {
			return nil, err
		}
		collectors = append(collectors, okxCollector)
	}
	return collectors, nil
}

// runtimeSymbols returns enabled exchange symbols for default rule API forms.
//
// Author: monsterfei
// Date: 2026-06-29
// @param cfg Runtime configuration.
// @returns Compact symbols from enabled exchanges.
func runtimeSymbols(cfg config.Config) []string {
	symbols := make([]string, 0, len(cfg.Binance.Symbols)+len(cfg.OKX.Symbols))
	if cfg.Binance.Enabled {
		symbols = append(symbols, cfg.Binance.Symbols...)
	}
	if cfg.OKX.Enabled {
		symbols = append(symbols, cfg.OKX.Symbols...)
	}
	return symbols
}

// buildNotificationSenders creates enabled notification senders from runtime config.
//
// Author: monsterfei
// Date: 2026-06-29
// @param cfg Runtime configuration.
// @param telegram Existing Telegram sender.
// @returns Channel-aware senders for the alert pipeline.
func buildNotificationSenders(cfg config.Config, telegram rule.Sender) []rule.NamedSender {
	senders := make([]rule.NamedSender, 0, 2)
	if cfg.Telegram.Enabled {
		senders = append(senders, rule.NewNamedSender("telegram", "default", telegram))
	}
	if cfg.Webhook.Enabled {
		channel := cfg.Webhook.Channel
		webhook := notifier.NewWebhookNotifier(cfg.Webhook.URL, channel, &http.Client{
			Timeout: time.Duration(cfg.Webhook.TimeoutSec) * time.Second,
		})
		senders = append(senders, rule.NewNamedSender(channel, cfg.Webhook.URL, webhook))
	}
	return senders
}

// buildSummaryService creates the optional AI market summary service from runtime config.
//
// Author: monsterfei
// Date: 2026-06-30
// @param cfg Runtime configuration.
// @param repos Storage repositories.
// @returns Summary service wired with bounded aggregation and configured generator.
func buildSummaryService(cfg config.Config, repos *storage.Repositories) summary.Service {
	aggregator := summary.NewAggregator(repos.Alerts, repos.MarketEvents, summary.Config{MaxItems: cfg.Summary.MaxItems})
	var generator summary.Generator = summary.NewTemplateGenerator(cfg.Summary.Disclaimer)
	if cfg.Summary.Provider == "openai_compatible" {
		generator = summary.NewOpenAICompatibleGenerator(summary.OpenAICompatibleConfig{
			BaseURL:    cfg.Summary.APIBaseURL,
			APIKey:     cfg.Summary.APIKey,
			Model:      cfg.Summary.Model,
			TimeoutSec: cfg.Summary.TimeoutSec,
			Disclaimer: cfg.Summary.Disclaimer,
		}, nil)
	}
	return summary.Service{
		Aggregator: aggregator,
		Generator:  generator,
		Store:      repos.MarketSummaries,
		Provider:   cfg.Summary.Provider,
		Window:     time.Duration(cfg.Summary.WindowSec) * time.Second,
	}
}

// collectorHealthAdapters converts runtime collectors to API health providers.
//
// Author: monsterfei
// Date: 2026-06-29
func collectorHealthAdapters(collectors []runtimeCollector) []api.CollectorStatusProvider {
	adapters := make([]api.CollectorStatusProvider, 0, len(collectors))
	for _, marketCollector := range collectors {
		adapters = append(adapters, collectorHealthAdapter{collector: marketCollector})
	}
	return adapters
}

type collectorHealthAdapter struct {
	collector interface {
		Status() collector.Status
	}
}

func (a collectorHealthAdapter) Status() api.CollectorStatus {
	status := a.collector.Status()
	return api.CollectorStatus{
		Name:          status.Name,
		Connected:     status.Connected,
		Reconnects:    status.Reconnects,
		LastEventAt:   optionalTime(status.LastEventAt),
		LastError:     status.LastError,
		Subscribed:    status.Subscribed,
		LastConnectAt: optionalTime(status.LastConnectAt),
	}
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

type dependencyHealthAdapter struct {
	name  string
	check func(context.Context) error
}

func (a dependencyHealthAdapter) Name() string {
	return a.name
}

func (a dependencyHealthAdapter) Check(ctx context.Context) error {
	return a.check(ctx)
}
