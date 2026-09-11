// Package forecasting defines forecast-ingestion application boundaries.
package forecasting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

// Stable provider errors let worker orchestration decide whether to defer,
// retry, alert, or quarantine without depending on an HTTP vendor adapter.
var (
	ErrInvalidRequest    = errors.New("invalid forecast provider request")
	ErrUnavailable       = errors.New("forecast provider unavailable")
	ErrRateLimited       = errors.New("forecast provider rate limited")
	ErrQuotaExhausted    = errors.New("forecast provider quota exhausted")
	ErrMalformedResponse = errors.New("malformed forecast provider response")
)

// FetchRequest asks for an inclusive-exclusive UTC window at one or more
// application-owned points. Providers may batch or chunk physical API calls.
type FetchRequest struct {
	Points   []forecast.Point
	StartsAt time.Time
	EndsAt   time.Time
}

// NewFetchRequest validates a provider-neutral hourly fetch request and copies
// its points so callers cannot mutate it after validation.
func NewFetchRequest(points []forecast.Point, startsAt, endsAt time.Time) (FetchRequest, error) {
	request := FetchRequest{
		Points:   append([]forecast.Point(nil), points...),
		StartsAt: startsAt.UTC(),
		EndsAt:   endsAt.UTC(),
	}
	if err := request.Validate(); err != nil {
		return FetchRequest{}, err
	}
	return request, nil
}

// Validate verifies the invariants required by every provider adapter.
func (request FetchRequest) Validate() error {
	if len(request.Points) == 0 {
		return fmt.Errorf("%w: at least one forecast point is required", ErrInvalidRequest)
	}
	if request.StartsAt.IsZero() || request.EndsAt.IsZero() || !request.StartsAt.Before(request.EndsAt) {
		return fmt.Errorf("%w: forecast window must have a start before its end", ErrInvalidRequest)
	}
	if !isHourAligned(request.StartsAt) || !isHourAligned(request.EndsAt) {
		return fmt.Errorf("%w: forecast window must align to whole hours", ErrInvalidRequest)
	}

	references := make(map[string]struct{}, len(request.Points))
	for _, point := range request.Points {
		if err := point.Validate(); err != nil {
			return fmt.Errorf("%w: validate forecast point: %w", ErrInvalidRequest, err)
		}
		if _, exists := references[point.Reference]; exists {
			return fmt.Errorf("%w: duplicate forecast point reference %q", ErrInvalidRequest, point.Reference)
		}
		references[point.Reference] = struct{}{}
	}
	return nil
}

func isHourAligned(value time.Time) bool {
	return value.Minute() == 0 && value.Second() == 0 && value.Nanosecond() == 0
}

// Provider is the driven port implemented by external forecast adapters.
// Implementations return only canonical SI values and opaque provenance.
type Provider interface {
	Fetch(context.Context, FetchRequest) (forecast.Batch, error)
}
