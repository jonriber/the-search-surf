// Command ingest-forecast runs one forecast ingestion cycle for an external scheduler.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
	"github.com/jonriber/the-search-surf/backend/internal/platform/forecastworker"
	"github.com/jonriber/the-search-surf/backend/internal/platform/openmeteo"
	"github.com/jonriber/the-search-surf/backend/internal/platform/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "forecast ingestion failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	config, err := forecastworker.ConfigFromEnvironment(os.LookupEnv)
	if err != nil {
		return err
	}
	operationContext, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	pool, err := pgxpool.New(operationContext, config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open forecast database: %w", err)
	}
	repository, err := postgres.NewForecastRepository(pool)
	if err != nil {
		pool.Close()
		return err
	}
	provider, err := openmeteo.New(config.OpenMeteo, http.DefaultClient, repository)
	if err != nil {
		pool.Close()
		return err
	}
	service, err := forecasting.NewService(openmeteo.ProviderID, provider, repository)
	if err != nil {
		pool.Close()
		return err
	}
	runErr := (forecastworker.Runner{
		Ingester: service,
		Output:   os.Stdout,
		Now:      time.Now,
		Horizon:  config.Horizon,
	}).Run(operationContext)
	pool.Close()
	return errors.Join(runErr, operationContext.Err())
}
