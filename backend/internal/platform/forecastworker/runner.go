package forecastworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
)

// Ingester is the application use case driven by the worker.
type Ingester interface {
	Ingest(context.Context, time.Time, time.Time) (forecasting.Result, error)
}

// Runner derives an hour-aligned window and reports a machine-readable result.
type Runner struct {
	Ingester Ingester
	Output   io.Writer
	Now      func() time.Time
	Horizon  time.Duration
}

// Run executes one bounded ingestion cycle for an external scheduler.
func (runner Runner) Run(ctx context.Context) error {
	if runner.Ingester == nil || runner.Output == nil || runner.Now == nil || runner.Horizon <= 0 {
		return errors.New("forecast worker dependencies and positive horizon are required")
	}
	startsAt := runner.Now().UTC().Truncate(time.Hour)
	endsAt := startsAt.Add(runner.Horizon)
	result, err := runner.Ingester.Ingest(ctx, startsAt, endsAt)
	if err != nil {
		return fmt.Errorf("ingest forecast window: %w", err)
	}
	status := "replayed"
	if result.Persisted {
		status = "persisted"
	}
	if err := json.NewEncoder(runner.Output).Encode(map[string]any{
		"status":    status,
		"starts_at": startsAt,
		"ends_at":   endsAt,
	}); err != nil {
		return fmt.Errorf("encode forecast worker result: %w", err)
	}
	return nil
}
