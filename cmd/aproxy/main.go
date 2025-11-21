package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"aproxy/internal/config"
	"aproxy/internal/pool"
	"aproxy/pkg/observability"
	my "aproxy/pkg/protocol/mysql"
	"aproxy/pkg/schema"
	"aproxy/pkg/session"
	"aproxy/pkg/sqlrewrite"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	configFile = flag.String("config", "configs/config.yaml", "Path to configuration file")
	version    = "dev"
	commit     = "none"
	date       = "unknown"
)

func main() {
	flag.Parse()

	fmt.Printf("AProxy %s (commit: %s, built: %s)\n", version, commit, date)

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		os.Exit(1)
	}

	logger, err := observability.NewLogger(
		cfg.Observability.LogLevel,
		cfg.Observability.LogFormat,
		cfg.Observability.RedactParameters,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting AProxy",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("date", date),
	)

	metrics := observability.NewMetrics()

	pgPool, err := pool.NewPool(&pool.Config{
		Host:        cfg.Postgres.Host,
		Port:        cfg.Postgres.Port,
		Database:    cfg.Postgres.Database,
		User:        cfg.Postgres.User,
		Password:    cfg.Postgres.Password,
		SSLMode:     cfg.Postgres.SSLMode,
		MaxPoolSize: cfg.Postgres.MaxPoolSize,
		Mode:        pool.ConnectionMode(cfg.Postgres.ConnectionMode),
	})
	if err != nil {
		logger.Fatal("Failed to create PostgreSQL pool", zap.Error(err))
	}
	defer pgPool.Close()

	logger.Info("PostgreSQL connection pool initialized",
		zap.String("host", cfg.Postgres.Host),
		zap.Int("port", cfg.Postgres.Port),
		zap.String("database", cfg.Postgres.Database),
		zap.String("mode", cfg.Postgres.ConnectionMode),
	)

	ctx := context.Background()
	if err := pgPool.Ping(ctx); err != nil {
		logger.Fatal("Failed to ping PostgreSQL", zap.Error(err))
	}
	logger.Info("PostgreSQL connection verified")

	// Initialize global schema cache
	if cfg.SchemaCache.Enabled {
		schema.InitGlobalCache(cfg.SchemaCache.TTL)
		logger.Info("Global schema cache initialized",
			zap.Duration("ttl", cfg.SchemaCache.TTL),
			zap.Int("max_entries", cfg.SchemaCache.MaxEntries),
		)
	}

	sessionMgr := session.NewManager()
	rewriter := sqlrewrite.NewRewriter(cfg.SQLRewrite.Enabled)

	handler := my.NewHandler(pgPool, sessionMgr, rewriter, metrics, logger, cfg.SQLRewrite.DebugSQL)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	go func() {
		metricsAddr := fmt.Sprintf(":%d", cfg.Observability.MetricsPort)
		logger.Info("Starting metrics server", zap.String("addr", metricsAddr))

		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := pgPool.Ping(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("PostgreSQL unhealthy"))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		if err := http.ListenAndServe(metricsAddr, nil); err != nil {
			logger.Error("Metrics server error", zap.Error(err))
		}
	}()

	go func() {
		logger.Info("Starting MySQL protocol server", zap.String("addr", addr))

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Fatal("Failed to create MySQL listener", zap.Error(err))
		}
		defer listener.Close()

		for {
			conn, err := listener.Accept()
			if err != nil {
				logger.Error("Failed to accept connection", zap.Error(err))
				continue
			}

			go func(c net.Conn) {
				defer c.Close()

				connHandler, err := handler.NewConnection(c)
				if err != nil {
					logger.Error("Failed to create connection handler", zap.Error(err))
					return
				}

				mysqlConn, err := server.NewConn(c, "root", "", connHandler)
				if err != nil {
					logger.Error("Failed to create MySQL connection", zap.Error(err))
					return
				}

				for {
					if err := mysqlConn.HandleCommand(); err != nil {
						logger.Debug("Connection closed", zap.Error(err))
						return
					}
				}
			}(conn)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan

	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	logger.Info("Closing active sessions")
	sessions := sessionMgr.GetAllSessions()
	for _, sess := range sessions {
		sessionMgr.RemoveSession(sess.ID)
	}

	logger.Info("Shutdown complete")
}
