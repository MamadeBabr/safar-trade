// Package main wires the application: configuration, database connection,
// dependency injection, routing, middleware, and graceful shutdown.
//
// Node comparison:
// In an Express app, this corresponds to your `index.ts` / `server.ts` file,
// where you initialize Prisma/pg, configure `app.use(...)`, and call `app.listen()`.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/internal/config"
	"github.com/hossen/safar-trade/internal/handler"
	"github.com/hossen/safar-trade/internal/ingestion/binance"
	"github.com/hossen/safar-trade/internal/ingestion/forexfactory"
	"github.com/hossen/safar-trade/internal/middleware"
	"github.com/hossen/safar-trade/internal/model"
	tfCandle "github.com/hossen/safar-trade/internal/processing/candle"
	"github.com/hossen/safar-trade/internal/repository"
	"github.com/hossen/safar-trade/internal/scheduler"
	"github.com/hossen/safar-trade/internal/service"
	redisStore "github.com/hossen/safar-trade/internal/storage/redis"
	"github.com/hossen/safar-trade/internal/ws"
	"github.com/hossen/safar-trade/migrations"
)

func main() {
	// 1. Load typed configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize structured logger (log/slog)
	// Node comparison: Like choosing between Pino JSON formatting in production vs pretty-print in dev.
	var logHandler slog.Handler
	logLevel := parseLogLevel(cfg.LogLevel)
	opts := &slog.HandlerOptions{Level: logLevel}

	if cfg.AppEnv == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	logger.Info("starting application",
		slog.String("env", cfg.AppEnv),
		slog.Int("port", cfg.Port),
	)

	// 3. Handle optional migration CLI flags (e.g. -migrate=status or -migrate=down)
	migrateFlag := flag.String("migrate", "up", "migration action: up, down, status")
	flag.Parse()

	if *migrateFlag == "down" {
		if err := migrations.Down(cfg.DatabaseURL); err != nil {
			logger.Error("migration rollback failed", slog.Any("error", err))
			os.Exit(1)
		}
		logger.Info("database rollback completed")
		return
	}
	if *migrateFlag == "status" {
		if err := migrations.Status(cfg.DatabaseURL); err != nil {
			logger.Error("migration status check failed", slog.Any("error", err))
			os.Exit(1)
		}
		return
	}

	// 4. Automatically run embedded database migrations on startup
	logger.Info("checking and applying database migrations...")
	if err := migrations.Up(cfg.DatabaseURL, logger); err != nil {
		logger.Error("failed to apply database migrations", slog.Any("error", err))
		os.Exit(1)
	}

	// 5. Setup root context that cancels on SIGINT (Ctrl+C) or SIGTERM (Docker/K8s stop signal)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop() // Cleans up signal listener when main exits

	// 4. Initialize PostgreSQL connection pool (pgxpool)
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to parse database connection URL", slog.Any("error", err))
		os.Exit(1)
	}
	// Tuned for 2GB RAM low-memory environment
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour

	// Use an initial ping context with a 5-second deadline
	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Second)
	defer cancelConnect()

	dbPool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		logger.Error("unable to connect to database pool", slog.Any("error", err))
		os.Exit(1)
	}
	defer dbPool.Close() // Drains connections cleanly on termination

	// Ping database to verify connection health
	if err := dbPool.Ping(connectCtx); err != nil {
		logger.Warn("database ping failed: database may be offline or starting up", slog.Any("error", err))
	} else {
		logger.Info("database connection established successfully")
	}

	// 5. Initialize Redis connection pool
	redisClient, err := redisStore.NewClient(ctx, cfg.RedisURL, logger)
	if err != nil {
		logger.Warn("redis connection initialized with warning: redis may be offline or starting up", slog.Any("error", err))
	} else {
		defer redisClient.Close()
	}
	candleStore := redisStore.NewCandleStore(redisClient, 0)

	// 5. Instantiate dependencies (Clean Architecture / Dependency Injection)
	validate := validator.New()

	// Note domain
	noteRepo := repository.NewNoteRepository(dbPool)
	noteService := service.NewNoteService(noteRepo)
	noteHandler := handler.NewNoteHandler(noteService, validate)

	// Forex Factory Ingestion domain
	economicEventRepo := repository.NewEconomicEventRepository(dbPool)
	ffClient, err := forexfactory.NewClient(logger)
	if err != nil {
		logger.Error("failed to initialize Forex Factory TLS client", slog.Any("error", err))
		os.Exit(1)
	}
	ffIngester := forexfactory.NewIngester(ffClient, economicEventRepo, logger, cfg.ForexFactoryJSONPath)
	ingestionHandler := handler.NewIngestionHandler(ffIngester, economicEventRepo, cfg.ForexFactoryJSONPath)

	// Auth, Subscription & Protected Market Data domains
	tfRepo := tfCandle.NewRepository(dbPool, redisClient)
	subStore := redisStore.NewSubscriptionStore(redisClient)
	userRepo := repository.NewUserRepository(dbPool)
	packageRepo := repository.NewPackageRepository(dbPool)
	subRepo := repository.NewSubscriptionRepository(dbPool)
	sessionRepo := repository.NewSessionRepository(dbPool)

	subService := service.NewSubscriptionService(subRepo, packageRepo, subStore, cfg.SubscriptionCacheTTL)
	authService := service.NewAuthService(userRepo, sessionRepo, subService, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)

	authHandler := handler.NewAuthHandler(authService, subService, userRepo)
	subHandler := handler.NewSubscriptionHandler(subService)
	dataHandler := handler.NewDataHandler(tfRepo, economicEventRepo)

	// WebSocket Real-time Hub & Streaming Handler
	wsHub := ws.NewHub(logger)
	go wsHub.Run(ctx)
	wsHandler := handler.NewWSHandler(wsHub, authService, subService, logger)

	// 6. Setup Router & Documentation (Standard Library Go 1.22+ ServeMux + Huma OpenAPI 3.1)
	mux := http.NewServeMux()
	noteHandler.RegisterRoutes(mux)
	ingestionHandler.RegisterRoutes(mux)
	wsHandler.RegisterRoutes(mux)

	// Configure Huma v2 with automatic interactive OpenAPI documentation at /docs
	humaConfig := huma.DefaultConfig("Safar Trade API", "1.0.0")
	if humaConfig.OpenAPI.Components == nil {
		humaConfig.OpenAPI.Components = &huma.Components{}
	}
	humaConfig.OpenAPI.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "Enter JWT access token with Bearer prefix",
		},
	}
	api := humago.New(mux, humaConfig)

	// Define Huma security and subscription middlewares
	humaAuthMW := huma.Middlewares{middleware.NewHumaAuthMiddleware(api, authService)}
	humaSubMW := huma.Middlewares{
		middleware.NewHumaAuthMiddleware(api, authService),
		middleware.NewHumaRequireSubscriptionMiddleware(api, subService),
	}

	authHandler.RegisterRoutes(api, humaAuthMW)
	subHandler.RegisterRoutes(api, humaAuthMW)
	dataHandler.RegisterRoutes(api, humaSubMW)

	// 7. Apply Middleware Chain
	appHandler := middleware.Chain(
		mux,
		middleware.Recoverer(logger),
		middleware.RequestID,
		middleware.Logger(logger),
		middleware.CORS(cfg.CORSOrigin),
	)

	// 8. Start Background Scheduler if enabled
	var sched *scheduler.Scheduler
	if cfg.EnableScheduler {
		sched = scheduler.NewScheduler(ffIngester, logger, cfg.IngestionInterval)
		sched.Start(ctx)
	}

	// 9. Start Binance Spot & Futures WebSocket Collector and Multi-Timeframe Processor if enabled
	var binanceCollector *binance.Collector
	var tfProcessor tfCandle.Processor

	if cfg.EnableBinance {
		var symbols []string
		if cfg.BinanceSymbols != "" {
			for _, s := range strings.Split(cfg.BinanceSymbols, ",") {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					symbols = append(symbols, trimmed)
				}
			}
		}

		binanceCollector = binance.NewCollector(
			binance.CollectorConfig{
				SpotWSURL:           cfg.BinanceSpotWSURL,
				FuturesWSURL:        cfg.BinanceFuturesWSURL,
				FuturesSymbolSuffix: "_FUTURES",
				FlushInterval:       cfg.BinanceFlushInterval,
				ExplicitSymbols:     symbols,
			},
			candleStore,
			logger,
		)

		// Initialize multi-timeframe candle processor (5m, 15m, 30m, 60m, 240m, 1d, 1w, 1M)
		tfProcessor = tfCandle.NewProcessor(tfRepo, redisClient, logger, cfg.BinanceFlushInterval)
		tfProcessor.Start(ctx)

		// Register listener: each candle update updates in-memory timeframe bars & Redis, and broadcasts live tick
		binanceCollector.RegisterCandleListener(func(candle *model.Candle) {
			tfProcessor.Process(candle)
			wsHub.BroadcastCandle(candle.Symbol, "1m", ws.CandlePayload{
				Symbol:    candle.Symbol,
				Timeframe: "1m",
				O:         candle.Open,
				H:         candle.High,
				L:         candle.Low,
				C:         candle.Close,
				V:         candle.Volume,
				T:         candle.OpenTime,
				IsClosed:  candle.IsClosed,
			})
		})

		// Listen to multi-timeframe bar updates (5m, 15m, 30m, 60m, etc.) and stream to WebSocket
		tfProcessor.RegisterListener(func(tf tfCandle.Timeframe, c *tfCandle.TimeframeCandle) {
			wsHub.BroadcastCandle(c.Symbol, string(tf), c)
		})

		go func() {
			if err := binanceCollector.Start(ctx); err != nil {
				logger.Error("failed to start Binance collector", slog.Any("error", err))
			}
		}()
	}

	// 9. Configure HTTP Server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      appHandler,
		ReadTimeout:  cfg.ServerTimeout,
		WriteTimeout: cfg.ServerTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// 10. Start server in a background Goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", slog.String("addr", server.Addr))
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 11. Wait for either an error or shutdown signal
	select {
	case err := <-serverErrors:
		logger.Error("server listener error", slog.Any("error", err))
		os.Exit(1)
	case <-ctx.Done():
		logger.Info("shutdown signal received, initiating graceful shutdown")
	}

	// 12. Graceful Shutdown
	if binanceCollector != nil {
		binanceCollector.Stop() // Flush cached candles and disconnect streams
	}
	if tfProcessor != nil {
		tfProcessor.Stop() // Flush any remaining finalized timeframe candles to database
	}
	if sched != nil {
		sched.Wait() // Wait for active background ingestion job to finish
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownPeriod)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced shutdown due to error", slog.Any("error", err))
		_ = server.Close()
	}

	logger.Info("server exited gracefully")
}

func parseLogLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
