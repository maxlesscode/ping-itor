// Command ping-itor is a lightweight web uptime monitor.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/maxlesscode/ping-itor/internal/checker"
	"github.com/maxlesscode/ping-itor/internal/notifier"
	"github.com/maxlesscode/ping-itor/internal/store"
	"github.com/maxlesscode/ping-itor/internal/web"
)

// version is injected at build time via -ldflags "-X main.version=$TAG".
var version = "dev"

type config struct {
	DatabasePath string
	HTTPAddr     string
	SMTP         notifier.Config
}

func loadConfig() config {
	cfg := config{
		DatabasePath: envOr("DATABASE_PATH", "./ping-itor.db"),
		HTTPAddr:     envOr("HTTP_ADDR", ":8080"),
		SMTP: notifier.Config{
			Host: os.Getenv("SMTP_HOST"),
			Port: envInt("SMTP_PORT", 587),
			User: os.Getenv("SMTP_USER"),
			Pass: os.Getenv("SMTP_PASS"),
			From: os.Getenv("SMTP_FROM"),
			To:   os.Getenv("SMTP_TO"),
		},
	}

	// If SMTP_TO is set, all other SMTP fields are required.
	if cfg.SMTP.To != "" {
		missing := []string{}
		if cfg.SMTP.Host == "" {
			missing = append(missing, "SMTP_HOST")
		}
		if cfg.SMTP.User == "" {
			missing = append(missing, "SMTP_USER")
		}
		if cfg.SMTP.Pass == "" {
			missing = append(missing, "SMTP_PASS")
		}
		if cfg.SMTP.From == "" {
			missing = append(missing, "SMTP_FROM")
		}
		if len(missing) > 0 {
			slog.Error("missing required environment variables", "vars", missing)
			os.Exit(1)
		}
	}

	return cfg
}

func main() {
	cfg := loadConfig()

	slog.Info("starting ping-itor",
		"version", version,
		"addr", cfg.HTTPAddr,
		"db", cfg.DatabasePath,
		"smtp_to", cfg.SMTP.To,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabasePath)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	n := notifier.New(cfg.SMTP)
	bc := web.NewBroadcaster()

	// Start the check loop in a goroutine tied to the root context (CC-2).
	go checker.Run(ctx, checker.RunInput{
		Store:    st,
		Notifier: n,
		Broadcast: func(r checker.CheckResult) {
			bc.Publish(r)
		},
		Interval: 60 * time.Second,
		Timeout:  10 * time.Second,
	})

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      web.NewHandler(web.HandlerInput{Store: st, Broadcaster: bc}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE connections are long-lived; no write deadline
		IdleTimeout:  120 * time.Second,
	}

	// Start HTTP server in background.
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Block until shutdown signal or server error.
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining connections")
	case err := <-serverErr:
		slog.Error("http server error", "err", err)
		os.Exit(1)
	}

	// Graceful shutdown with 10-second deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}

	slog.Info("ping-itor stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Error("invalid integer env var", "key", key, "value", v)
		os.Exit(1)
	}
	return n
}
