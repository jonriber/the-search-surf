package forecasting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

// ErrNoActivePoints means ingestion has no configured provider sampling work.
var ErrNoActivePoints = errors.New("no active forecast points")

// Repository is the persistence boundary owned by forecast ingestion. Save is
// atomic and returns false when an equivalent batch was already committed.
type Repository interface {
	ListActivePoints(context.Context, string) ([]forecast.Point, error)
	Save(context.Context, FetchRequest, forecast.Batch) (bool, error)
	Quarantine(context.Context, Rejection) error
}

// Rejection is safe diagnostic evidence for an upstream payload that could not
// be normalized. Bodies and request URLs are deliberately excluded.
type Rejection struct {
	ProviderID       string
	PointReference   string
	Component        forecast.Component
	FetchedAt        time.Time
	SHA256           string
	StorageReference string
	Reason           string
}

// Result distinguishes a new durable batch from an idempotent replay.
type Result struct {
	Persisted bool
}

// Service coordinates point selection, the provider port, validation, and one
// atomic persistence operation.
type Service struct {
	providerID string
	provider   Provider
	repository Repository
}

// NewService constructs the forecast-ingestion use case.
func NewService(providerID string, provider Provider, repository Repository) (*Service, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, errors.New("forecast provider ID is required")
	}
	if provider == nil {
		return nil, errors.New("forecast provider is required")
	}
	if repository == nil {
		return nil, errors.New("forecast repository is required")
	}
	return &Service{providerID: providerID, provider: provider, repository: repository}, nil
}

// Ingest fetches, validates, and atomically stores one forecast window.
func (service *Service) Ingest(ctx context.Context, startsAt, endsAt time.Time) (Result, error) {
	points, err := service.repository.ListActivePoints(ctx, service.providerID)
	if err != nil {
		return Result{}, fmt.Errorf("list active forecast points: %w", err)
	}
	if len(points) == 0 {
		return Result{}, ErrNoActivePoints
	}
	request, err := NewFetchRequest(points, startsAt, endsAt)
	if err != nil {
		return Result{}, err
	}

	batch, err := service.provider.Fetch(ctx, request)
	if err != nil {
		var malformed *MalformedResponseError
		if errors.As(err, &malformed) {
			rejection := Rejection{
				ProviderID:       service.providerID,
				PointReference:   malformed.PointReference,
				Component:        malformed.Component,
				FetchedAt:        malformed.FetchedAt,
				SHA256:           malformed.SHA256,
				StorageReference: malformed.StorageReference,
				Reason:           malformed.Reason,
			}
			if quarantineErr := service.repository.Quarantine(ctx, rejection); quarantineErr != nil {
				return Result{}, errors.Join(err, fmt.Errorf("quarantine malformed forecast payload: %w", quarantineErr))
			}
		}
		return Result{}, err
	}
	if err := ValidateBatch(request, batch, service.providerID); err != nil {
		return Result{}, fmt.Errorf("reject forecast batch: %w", err)
	}
	persisted, err := service.repository.Save(ctx, request, batch)
	if err != nil {
		return Result{}, fmt.Errorf("persist forecast batch: %w", err)
	}
	return Result{Persisted: persisted}, nil
}
