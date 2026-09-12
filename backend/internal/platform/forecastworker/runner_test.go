package forecastworker

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
)

func TestRunnerAlignsWindowAndReportsPersistence(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	ingester := &ingesterStub{result: forecasting.Result{Persisted: true}}
	runner := Runner{
		Ingester: ingester,
		Output:   &output,
		Now: func() time.Time {
			return time.Date(2026, time.September, 12, 12, 37, 0, 0, time.FixedZone("WEST", 3600))
		},
		Horizon: 48 * time.Hour,
	}
	if err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantStart := time.Date(2026, time.September, 12, 11, 0, 0, 0, time.UTC)
	if !ingester.startsAt.Equal(wantStart) || !ingester.endsAt.Equal(wantStart.Add(48*time.Hour)) {
		t.Fatalf("window = %s - %s", ingester.startsAt, ingester.endsAt)
	}
	if !strings.Contains(output.String(), `"status":"persisted"`) {
		t.Fatalf("output = %q", output.String())
	}
}

type ingesterStub struct {
	result   forecasting.Result
	startsAt time.Time
	endsAt   time.Time
}

func (stub *ingesterStub) Ingest(_ context.Context, startsAt, endsAt time.Time) (forecasting.Result, error) {
	stub.startsAt = startsAt
	stub.endsAt = endsAt
	return stub.result, nil
}
