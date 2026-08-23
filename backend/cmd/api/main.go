// Package main assembles and runs The Search API process.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/platform/config"
	"github.com/jonriber/the-search-surf/backend/internal/platform/healthcheck"
	"github.com/jonriber/the-search-surf/backend/internal/platform/httpserver"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.FromEnvironment(os.LookupEnv)
	if err != nil {
		logger.Error("invalid application configuration", "error", err)
		os.Exit(1)
	}
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := healthcheck.Check(ctx, cfg.HTTPAddress, http.DefaultClient); err != nil {
			logger.Error("api healthcheck failed", "error", err)
			os.Exit(1)
		}
		return
	}

	handler := httpserver.NewHandler(httpserver.HandlerOptions{
		Logger: logger,
		Version: httpserver.Version{
			Release: version,
			Commit:  commit,
		},
	})

	server := httpserver.New(cfg, handler)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api server starting", "address", cfg.HTTPAddress, "version", version, "commit", commit)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("api server shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("api server shutdown failed", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("api server stopped")
}
