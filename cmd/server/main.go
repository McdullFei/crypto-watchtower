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
	"github.com/renfei198727/crypto-watchtower/internal/collector"
	"github.com/renfei198727/crypto-watchtower/internal/config"
	"github.com/renfei198727/crypto-watchtower/internal/eventbus"
	"github.com/renfei198727/crypto-watchtower/internal/model"
	"github.com/renfei198727/crypto-watchtower/internal/notifier"
	"github.com/renfei198727/crypto-watchtower/internal/rule"
	"github.com/renfei198727/crypto-watchtower/internal/scheduler"
	"github.com/renfei198727/crypto-watchtower/internal/storage"
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
	pipeline := rule.NewPipeline(engine, repos, redisClient, tg)

	go func() {
		sub := bus.Subscribe(ctx)
		for event := range sub {
			if err := pipeline.HandleEvent(ctx, event); err != nil {
				slog.Error("handle event", "err", err, "event_id", event.ID)
			}
		}
	}()

	spotCollector := collector.NewBinanceWSCollector(collector.MarketTypeSpot, cfg.Binance.SpotWSBaseURL, bus)
	futuresCollector := collector.NewBinanceWSCollector(collector.MarketTypeFutures, cfg.Binance.FuturesWSBaseURL, bus)
	if err := spotCollector.Subscribe(cfg.Binance.Symbols); err != nil {
		slog.Error("subscribe spot collector", "err", err)
		os.Exit(1)
	}
	if err := futuresCollector.Subscribe(cfg.Binance.Symbols); err != nil {
		slog.Error("subscribe futures collector", "err", err)
		os.Exit(1)
	}
	go func() {
		if err := spotCollector.Start(ctx); err != nil && ctx.Err() == nil {
			slog.Error("spot collector stopped", "err", err)
		}
	}()
	go func() {
		if err := futuresCollector.Start(ctx); err != nil && ctx.Err() == nil {
			slog.Error("futures collector stopped", "err", err)
		}
	}()

	fundingFetcher := collector.NewFundingFetcher(cfg.Binance.FuturesRESTBaseURL, cfg.Binance.Symbols, bus)
	fundingJob := scheduler.NewFundingJob(fundingFetcher, time.Duration(cfg.Scheduler.FundingIntervalSec)*time.Second)
	go fundingJob.Start(ctx)

	if cfg.Telegram.Enabled && cfg.Telegram.Mode == "polling" {
		poller := notifier.NewTelegramPoller(
			notifier.NewTelegramClient(cfg.Telegram.BotToken, "", nil),
			repos.Users,
			ruleService,
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
		APIBearerToken: cfg.API.BearerToken,
		Symbols:        cfg.Binance.Symbols,
		RuleConfig:     cfg.Rules,
		Rules:          ruleService,
		Admin:          adminService,
		Telegram:       tg,
		Collectors: []api.CollectorStatusProvider{
			collectorHealthAdapter{collector: spotCollector},
			collectorHealthAdapter{collector: futuresCollector},
		},
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
