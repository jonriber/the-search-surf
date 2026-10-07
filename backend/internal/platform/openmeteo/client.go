package openmeteo

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

// HTTPClient is the narrow outbound transport needed by the adapter.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// QuotaLedger coordinates physical request budgets across worker replicas.
type QuotaLedger interface {
	ReserveProviderAttempt(context.Context, string, string, time.Time, int64, int64) (bool, error)
}

// Client implements forecasting.Provider for Open-Meteo.
type Client struct {
	config  Config
	http    HTTPClient
	quota   QuotaLedger
	now     func() time.Time
	sleep   func(context.Context, time.Duration) error
	breaker *circuitBreaker
}

// New constructs an Open-Meteo provider adapter.
func New(config Config, httpClient HTTPClient, quota QuotaLedger) (*Client, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if httpClient == nil {
		return nil, errors.New("Open-Meteo HTTP client is required")
	}
	if quota == nil {
		return nil, errors.New("Open-Meteo quota ledger is required")
	}
	return &Client{
		config:  config,
		http:    httpClient,
		quota:   quota,
		now:     time.Now,
		sleep:   sleepContext,
		breaker: newCircuitBreaker(5, time.Minute),
	}, nil
}

// Fetch returns one normalized, metadata-bracketed component batch.
func (client *Client) Fetch(ctx context.Context, request forecasting.FetchRequest) (forecast.Batch, error) {
	if err := request.Validate(); err != nil {
		return forecast.Batch{}, err
	}
	logicalContext, cancel := context.WithTimeout(ctx, client.config.LogicalTimeout)
	defer cancel()

	series := initializeSeries(request)
	digests := make([]forecast.PayloadDigest, 0, len(componentSpecs))
	for _, spec := range componentSpecs {
		metadataBefore, err := client.fetchMetadata(logicalContext, spec)
		if err != nil {
			return forecast.Batch{}, err
		}
		body, err := client.fetchData(logicalContext, spec, request)
		if err != nil {
			return forecast.Batch{}, err
		}
		digest := sha256.Sum256(body)
		metadataAfter, err := client.fetchMetadata(logicalContext, spec)
		if err != nil {
			return forecast.Batch{}, err
		}
		if metadataBefore != metadataAfter {
			return forecast.Batch{}, fmt.Errorf("%w: %s model metadata changed during fetch", forecasting.ErrUnavailable, spec.component)
		}
		if client.now().UTC().Sub(metadataAfter.availableAt) < client.config.AvailabilityLag {
			return forecast.Batch{}, fmt.Errorf("%w: %s model run is not stable yet", forecasting.ErrUnavailable, spec.component)
		}
		if err := decodeComponent(body, request, spec, metadataAfter, series); err != nil {
			return forecast.Batch{}, &forecasting.MalformedResponseError{
				Component: spec.component,
				FetchedAt: client.now().UTC(),
				SHA256:    hex.EncodeToString(digest[:]),
				Reason:    err.Error(),
				Cause:     err,
			}
		}
		digests = append(digests, forecast.PayloadDigest{
			Component: spec.component,
			SHA256:    hex.EncodeToString(digest[:]),
		})
	}

	return forecast.Batch{
		ProviderID:            ProviderID,
		FetchedAt:             client.now().UTC(),
		TransformationVersion: TransformationVersion,
		Attribution: forecast.Attribution{
			Text:       "Weather data by Open-Meteo.com",
			URL:        "https://open-meteo.com/",
			License:    "CC BY 4.0",
			LicenseURL: "https://creativecommons.org/licenses/by/4.0/",
		},
		PayloadDigests: digests,
		Series:         series,
	}, nil
}

func (client *Client) fetchMetadata(ctx context.Context, spec componentSpec) (modelMetadata, error) {
	endpoint, err := url.Parse(spec.metadataURL(client.config))
	if err != nil {
		return modelMetadata{}, fmt.Errorf("%w: configure %s metadata endpoint: %w", forecasting.ErrInvalidRequest, spec.component, err)
	}
	query := endpoint.Query()
	query.Set("cache_buster", strconv.FormatInt(client.now().UnixMilli(), 10))
	endpoint.RawQuery = query.Encode()
	body, err := client.get(ctx, endpoint.String(), spec.component)
	if err != nil {
		return modelMetadata{}, err
	}
	metadata, err := decodeMetadata(body)
	if err != nil {
		digest := sha256.Sum256(body)
		return modelMetadata{}, &forecasting.MalformedResponseError{
			Component: spec.component,
			FetchedAt: client.now().UTC(),
			SHA256:    hex.EncodeToString(digest[:]),
			Reason:    err.Error(),
			Cause:     err,
		}
	}
	return metadata, nil
}

func (client *Client) fetchData(ctx context.Context, spec componentSpec, request forecasting.FetchRequest) ([]byte, error) {
	endpoint, err := url.Parse(spec.dataURL(client.config))
	if err != nil {
		return nil, fmt.Errorf("%w: configure %s endpoint: %w", forecasting.ErrInvalidRequest, spec.component, err)
	}
	query := endpoint.Query()
	latitudes := make([]string, 0, len(request.Points))
	longitudes := make([]string, 0, len(request.Points))
	for _, point := range request.Points {
		latitudes = append(latitudes, strconv.FormatFloat(point.Latitude, 'f', -1, 64))
		longitudes = append(longitudes, strconv.FormatFloat(point.Longitude, 'f', -1, 64))
	}
	query.Set("latitude", strings.Join(latitudes, ","))
	query.Set("longitude", strings.Join(longitudes, ","))
	query.Set("hourly", strings.Join(spec.variables, ","))
	query.Set("models", spec.model)
	query.Set("timezone", "GMT")
	query.Set("start_date", request.StartsAt.Format(time.DateOnly))
	query.Set("end_date", request.EndsAt.Add(-time.Nanosecond).Format(time.DateOnly))
	if spec.component == forecast.ComponentWind {
		query.Set("wind_speed_unit", "ms")
	} else {
		query.Set("cell_selection", "sea")
	}
	endpoint.RawQuery = query.Encode()
	return client.get(ctx, endpoint.String(), spec.component)
}

func (client *Client) get(ctx context.Context, endpoint string, component forecast.Component) ([]byte, error) {
	breakerKey := circuitKey(endpoint)
	if !client.breaker.allow(breakerKey, client.now()) {
		return nil, fmt.Errorf("%w: circuit open for %s", forecasting.ErrUnavailable, component)
	}
	var terminal error
	for attempt := 0; attempt <= client.config.MaximumRetries; attempt++ {
		reserved, err := client.quota.ReserveProviderAttempt(
			ctx,
			ProviderID,
			client.config.AccountScope,
			client.now().UTC(),
			client.config.DailyLimit,
			client.config.MonthlyLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("reserve Open-Meteo quota: %w", err)
		}
		if !reserved {
			return nil, forecasting.ErrQuotaExhausted
		}

		body, retryable, retryAfter, err := client.attempt(ctx, endpoint)
		if err == nil {
			client.breaker.success(breakerKey)
			return body, nil
		}
		terminal = err
		if errors.Is(err, forecasting.ErrMalformedResponse) {
			digest := sha256.Sum256(body)
			return nil, &forecasting.MalformedResponseError{
				Component: component,
				FetchedAt: client.now().UTC(),
				SHA256:    hex.EncodeToString(digest[:]),
				Reason:    err.Error(),
				Cause:     err,
			}
		}
		if !retryable || attempt == client.config.MaximumRetries {
			break
		}
		backoff := client.backoff(attempt)
		if retryAfter > backoff {
			backoff = retryAfter
		}
		if err := client.sleep(ctx, backoff); err != nil {
			return nil, err
		}
	}
	client.breaker.failure(breakerKey, client.now())
	return nil, terminal
}

func (client *Client) attempt(ctx context.Context, endpoint string) ([]byte, bool, time.Duration, error) {
	physicalContext, cancel := context.WithTimeout(ctx, client.config.PhysicalTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(physicalContext, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, 0, fmt.Errorf("%w: create provider request: %w", forecasting.ErrInvalidRequest, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("User-Agent", "the-search-surf/forecast-ingester")
	response, err := client.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, 0, ctx.Err()
		}
		return nil, true, 0, fmt.Errorf("%w: provider request failed: %w", forecasting.ErrUnavailable, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, client.config.MaximumResponseSize+1))
	if err != nil {
		return nil, true, 0, fmt.Errorf("%w: read provider response: %w", forecasting.ErrUnavailable, err)
	}
	if int64(len(body)) > client.config.MaximumResponseSize {
		return body, false, 0, fmt.Errorf("%w: provider response exceeds size limit", forecasting.ErrMalformedResponse)
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if mediaType := response.Header.Get("Content-Type"); mediaType != "" && !strings.Contains(mediaType, "application/json") {
			return body, false, 0, fmt.Errorf("%w: provider response is not JSON", forecasting.ErrMalformedResponse)
		}
		return body, false, 0, nil
	}
	if response.StatusCode == http.StatusTooManyRequests {
		return nil, true, parseRetryAfter(response.Header.Get("Retry-After"), client.now()), forecasting.ErrRateLimited
	}
	if response.StatusCode == http.StatusRequestTimeout || response.StatusCode >= 500 {
		return nil, true, 0, forecasting.ErrUnavailable
	}
	return nil, false, 0, fmt.Errorf("%w: provider returned HTTP %d", forecasting.ErrUnavailable, response.StatusCode)
}

func (client *Client) backoff(attempt int) time.Duration {
	maximum := client.config.InitialBackoff << attempt
	if maximum > client.config.MaximumBackoff {
		maximum = client.config.MaximumBackoff
	}
	if maximum <= 0 {
		return 0
	}
	randomValue, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(maximum)+1))
	if err != nil {
		return maximum
	}
	return time.Duration(randomValue.Int64())
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if instant, err := http.ParseTime(value); err == nil && instant.After(now) {
		return instant.Sub(now)
	}
	return 0
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func circuitKey(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	return parsed.Scheme + "://" + parsed.Host + parsed.Path
}

type circuitState struct {
	failures      int
	openUntil     time.Time
	probeInFlight bool
}

type circuitBreaker struct {
	mutex     sync.Mutex
	threshold int
	openFor   time.Duration
	states    map[string]circuitState
}

func newCircuitBreaker(threshold int, openFor time.Duration) *circuitBreaker {
	return &circuitBreaker{threshold: threshold, openFor: openFor, states: make(map[string]circuitState)}
}

func (breaker *circuitBreaker) allow(endpoint string, now time.Time) bool {
	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()
	state := breaker.states[endpoint]
	if state.openUntil.IsZero() {
		return true
	}
	if now.Before(state.openUntil) || state.probeInFlight {
		return false
	}
	state.probeInFlight = true
	breaker.states[endpoint] = state
	return true
}

func (breaker *circuitBreaker) success(endpoint string) {
	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()
	delete(breaker.states, endpoint)
}

func (breaker *circuitBreaker) failure(endpoint string, now time.Time) {
	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()
	state := breaker.states[endpoint]
	state.failures++
	state.probeInFlight = false
	if state.failures >= breaker.threshold {
		state.openUntil = now.Add(breaker.openFor)
	}
	breaker.states[endpoint] = state
}
