package forecasting

import (
	"context"
	"errors"
	"testing"

	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

func TestServicePersistsCanonicalBatchAndReportsReplay(t *testing.T) {
	request := validRequest(t)
	repository := &repositoryStub{points: request.Points, persisted: true}
	provider := &providerStub{batch: validBatch(request)}
	service, err := NewService("open-meteo", provider, repository)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Ingest(context.Background(), request.StartsAt, request.EndsAt)
	if err != nil || !result.Persisted {
		t.Fatalf("Ingest() = (%+v, %v), want persisted", result, err)
	}
	if provider.calls != 1 || repository.saveCalls != 1 {
		t.Fatalf("calls = provider:%d save:%d, want 1/1", provider.calls, repository.saveCalls)
	}

	repository.persisted = false
	result, err = service.Ingest(context.Background(), request.StartsAt, request.EndsAt)
	if err != nil || result.Persisted {
		t.Fatalf("replayed Ingest() = (%+v, %v), want non-persisted success", result, err)
	}
}

func TestServiceQuarantinesMalformedProviderEvidence(t *testing.T) {
	request := validRequest(t)
	failure := &MalformedResponseError{
		PointReference: request.Points[0].Reference,
		Component:      forecast.ComponentWaves,
		FetchedAt:      request.StartsAt,
		SHA256:         repeatedHex('d'),
		Reason:         "hourly arrays have different lengths",
	}
	repository := &repositoryStub{points: request.Points}
	service, err := NewService("open-meteo", &providerStub{err: failure}, repository)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Ingest(context.Background(), request.StartsAt, request.EndsAt)
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("Ingest() error = %v, want ErrMalformedResponse", err)
	}
	if repository.quarantineCalls != 1 || repository.rejection.SHA256 != failure.SHA256 || repository.saveCalls != 0 {
		t.Fatalf("repository state = %+v", repository)
	}
}

func TestServiceRejectsInvalidProviderBatchBeforePersistence(t *testing.T) {
	request := validRequest(t)
	batch := validBatch(request)
	batch.Series[0].Hours[0].WaveHeightMetres = available(-1)
	repository := &repositoryStub{points: request.Points}
	service, err := NewService("open-meteo", &providerStub{batch: batch}, repository)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Ingest(context.Background(), request.StartsAt, request.EndsAt)
	if !errors.Is(err, ErrInvalidBatch) || repository.saveCalls != 0 {
		t.Fatalf("Ingest() error = %v, save calls = %d", err, repository.saveCalls)
	}
}

func TestServiceStopsWhenNoPointsAreActive(t *testing.T) {
	provider := &providerStub{}
	service, err := NewService("open-meteo", provider, &repositoryStub{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Ingest(context.Background(), validRequest(t).StartsAt, validRequest(t).EndsAt)
	if !errors.Is(err, ErrNoActivePoints) || provider.calls != 0 {
		t.Fatalf("Ingest() error = %v, provider calls = %d", err, provider.calls)
	}
}

type providerStub struct {
	batch forecast.Batch
	err   error
	calls int
}

func (stub *providerStub) Fetch(_ context.Context, _ FetchRequest) (forecast.Batch, error) {
	stub.calls++
	return stub.batch, stub.err
}

type repositoryStub struct {
	points          []forecast.Point
	persisted       bool
	err             error
	saveCalls       int
	quarantineCalls int
	rejection       Rejection
}

func (stub *repositoryStub) ListActivePoints(context.Context, string) ([]forecast.Point, error) {
	return stub.points, stub.err
}

func (stub *repositoryStub) Save(context.Context, FetchRequest, forecast.Batch) (bool, error) {
	stub.saveCalls++
	return stub.persisted, stub.err
}

func (stub *repositoryStub) Quarantine(_ context.Context, rejection Rejection) error {
	stub.quarantineCalls++
	stub.rejection = rejection
	return stub.err
}
