package openmeteo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

func TestFetchNormalizesAllComponentsWithProvenance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(contractHandler(t, nil)))
	defer server.Close()
	quota := &quotaStub{allowed: true}
	client := newTestClient(t, server.URL, quota)
	request := testRequest(t)

	batch, err := client.Fetch(context.Background(), request)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if err := forecasting.ValidateBatch(request, batch, ProviderID); err != nil {
		t.Fatalf("ValidateBatch() error = %v", err)
	}
	if quota.calls != 9 {
		t.Fatalf("quota reservations = %d, want 9 physical requests", quota.calls)
	}
	hour := batch.Series[0].Hours[0]
	if hour.WaveHeightMetres.Value != 1.7 || hour.WindSpeedMetresPerSecond.Value != 4.2 ||
		hour.SeaLevelHeightMetresAboveMSL.Value != 0.6 {
		t.Fatalf("normalized hour = %+v", hour)
	}
	if hour.SwellHeightMetres.Available {
		t.Fatal("provider null became an available swell height")
	}
	if batch.Series[0].Sources[0].SampledLongitude == batch.Series[0].Sources[1].SampledLongitude {
		t.Fatal("component-specific sampled grids were collapsed")
	}
	if len(batch.PayloadDigests) != 3 {
		t.Fatalf("payload digests = %d, want 3", len(batch.PayloadDigests))
	}
}

func TestFetchReturnsChecksumEvidenceForMalformedPayload(t *testing.T) {
	badBody := strings.Replace(waveBody, `"wave_period":[11,12]`, `"wave_period":[11]`, 1)
	server := httptest.NewServer(http.HandlerFunc(contractHandler(t, func(request *http.Request) (string, bool) {
		if request.URL.Path == "/marine" && request.URL.Query().Get("models") == "meteofrance_wave" {
			return badBody, true
		}
		return "", false
	})))
	defer server.Close()
	client := newTestClient(t, server.URL, &quotaStub{allowed: true})

	_, err := client.Fetch(context.Background(), testRequest(t))
	if !errors.Is(err, forecasting.ErrMalformedResponse) {
		t.Fatalf("Fetch() error = %v, want ErrMalformedResponse", err)
	}
	var malformed *forecasting.MalformedResponseError
	if !errors.As(err, &malformed) {
		t.Fatalf("Fetch() error type = %T", err)
	}
	wantDigest := sha256.Sum256([]byte(badBody))
	if malformed.Component != forecast.ComponentWaves || malformed.SHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("malformed evidence = %+v", malformed)
	}
}

func TestFetchRejectsMetadataRollover(t *testing.T) {
	metadataCalls := 0
	server := httptest.NewServer(http.HandlerFunc(contractHandler(t, func(request *http.Request) (string, bool) {
		if request.URL.Path == "/metadata/waves" {
			metadataCalls++
			if metadataCalls == 2 {
				return `{"last_run_initialisation_time":1789167600,"last_run_availability_time":1789173000,"temporal_resolution_seconds":10800}`, true
			}
		}
		return "", false
	})))
	defer server.Close()
	client := newTestClient(t, server.URL, &quotaStub{allowed: true})

	_, err := client.Fetch(context.Background(), testRequest(t))
	if !errors.Is(err, forecasting.ErrUnavailable) || metadataCalls != 2 {
		t.Fatalf("Fetch() = error:%v metadata calls:%d", err, metadataCalls)
	}
}

func TestFetchRetriesOnlyWithinConfiguredBound(t *testing.T) {
	var mutex sync.Mutex
	metadataAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		if request.URL.Path == "/metadata/waves" {
			metadataAttempts++
			if metadataAttempts <= 2 {
				mutex.Unlock()
				response.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		mutex.Unlock()
		contractHandler(t, nil)(response, request)
	}))
	defer server.Close()
	quota := &quotaStub{allowed: true}
	client := newTestClient(t, server.URL, quota)
	client.sleep = func(context.Context, time.Duration) error { return nil }

	if _, err := client.Fetch(context.Background(), testRequest(t)); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if metadataAttempts != 4 {
		t.Fatalf("wave metadata attempts = %d, want 4 (three before plus one after)", metadataAttempts)
	}
	if quota.calls != 11 {
		t.Fatalf("quota reservations = %d, want 11 including retries", quota.calls)
	}
}

func TestFetchStopsBeforeHTTPWhenQuotaIsExhausted(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { serverCalls++ }))
	defer server.Close()
	client := newTestClient(t, server.URL, &quotaStub{})

	_, err := client.Fetch(context.Background(), testRequest(t))
	if !errors.Is(err, forecasting.ErrQuotaExhausted) || serverCalls != 0 {
		t.Fatalf("Fetch() = error:%v server calls:%d", err, serverCalls)
	}
}

func TestFetchClassifiesHTTPFailuresAndBoundsRetries(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantError    error
		wantAttempts int
	}{
		{name: "invalid upstream request", status: http.StatusBadRequest, wantError: forecasting.ErrUnavailable, wantAttempts: 1},
		{name: "rate limited", status: http.StatusTooManyRequests, wantError: forecasting.ErrRateLimited, wantAttempts: 3},
		{name: "server unavailable", status: http.StatusServiceUnavailable, wantError: forecasting.ErrUnavailable, wantAttempts: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				attempts++
				response.WriteHeader(test.status)
			}))
			defer server.Close()
			client := newTestClient(t, server.URL, &quotaStub{allowed: true})
			client.sleep = func(context.Context, time.Duration) error { return nil }

			_, err := client.Fetch(context.Background(), testRequest(t))
			if !errors.Is(err, test.wantError) || attempts != test.wantAttempts {
				t.Fatalf("Fetch() = error:%v attempts:%d, want %v/%d", err, attempts, test.wantError, test.wantAttempts)
			}
		})
	}
}

func newTestClient(t *testing.T, serverURL string, quota QuotaLedger) *Client {
	t.Helper()
	config := DefaultConfig()
	config.ForecastURL = serverURL + "/forecast"
	config.MarineURL = serverURL + "/marine"
	config.WaveMetadataURL = serverURL + "/metadata/waves"
	config.WindMetadataURL = serverURL + "/metadata/wind"
	config.SeaLevelMetadataURL = serverURL + "/metadata/sea-level"
	config.InitialBackoff = 0
	config.MaximumBackoff = 0
	client, err := New(config, http.DefaultClient, quota)
	if err != nil {
		t.Fatal(err)
	}
	client.now = func() time.Time {
		return time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)
	}
	return client
}

func testRequest(t *testing.T) forecasting.FetchRequest {
	t.Helper()
	point, err := forecast.NewPoint("11111111-1111-4111-8111-111111111111", -9.417, 38.963)
	if err != nil {
		t.Fatal(err)
	}
	request, err := forecasting.NewFetchRequest(
		[]forecast.Point{point},
		time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC),
		time.Date(2026, time.September, 12, 14, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func contractHandler(t *testing.T, override func(*http.Request) (string, bool)) http.HandlerFunc {
	t.Helper()
	return func(response http.ResponseWriter, request *http.Request) {
		if override != nil {
			if body, handled := override(request); handled {
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(body))
				return
			}
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/metadata/waves", "/metadata/wind", "/metadata/sea-level":
			_, _ = response.Write([]byte(metadataBody))
		case "/marine":
			assertCommonQuery(t, request.URL.Query())
			if request.URL.Query().Get("cell_selection") != "sea" {
				t.Errorf("cell_selection = %q", request.URL.Query().Get("cell_selection"))
			}
			switch request.URL.Query().Get("models") {
			case "meteofrance_wave":
				_, _ = response.Write([]byte(waveBody))
			case "meteofrance_currents":
				_, _ = response.Write([]byte(seaLevelBody))
			default:
				t.Errorf("unexpected marine model %q", request.URL.Query().Get("models"))
				response.WriteHeader(http.StatusBadRequest)
			}
		case "/forecast":
			assertCommonQuery(t, request.URL.Query())
			if request.URL.Query().Get("models") != "ecmwf_ifs" || request.URL.Query().Get("wind_speed_unit") != "ms" {
				t.Errorf("wind query = %s", request.URL.RawQuery)
			}
			_, _ = response.Write([]byte(windBody))
		default:
			http.NotFound(response, request)
		}
	}
}

func assertCommonQuery(t *testing.T, query url.Values) {
	t.Helper()
	want := map[string]string{
		"latitude": "38.963", "longitude": "-9.417", "timezone": "GMT",
		"start_date": "2026-09-12", "end_date": "2026-09-12",
	}
	for key, value := range want {
		if query.Get(key) != value {
			t.Errorf("query %s = %q, want %q", key, query.Get(key), value)
		}
	}
}

type quotaStub struct {
	allowed bool
	calls   int
}

func (stub *quotaStub) ReserveProviderAttempt(
	context.Context,
	string,
	string,
	time.Time,
	int64,
	int64,
) (bool, error) {
	stub.calls++
	return stub.allowed, nil
}

const metadataBody = `{"last_run_initialisation_time":1789142400,"last_run_availability_time":1789147800,"temporal_resolution_seconds":3600,"future_field":"accepted"}`

const waveBody = `{
  "latitude":38.96,
  "longitude":-9.42,
  "utc_offset_seconds":0,
  "hourly_units":{"wave_height":"m","wave_direction":"°","wave_period":"s","swell_wave_height":"m","swell_wave_direction":"°","swell_wave_period":"s"},
  "hourly":{"time":["2026-09-12T12:00","2026-09-12T13:00"],"wave_height":[1.7,1.8],"wave_direction":[287,289],"wave_period":[11,12],"swell_wave_height":[null,1.3],"swell_wave_direction":[null,292],"swell_wave_period":[null,13]}
}`

const windBody = `{
  "latitude":38.97,
  "longitude":-9.41,
  "utc_offset_seconds":0,
  "hourly_units":{"wind_speed_10m":"m/s","wind_direction_10m":"°","wind_gusts_10m":"m/s"},
  "hourly":{"time":["2026-09-12T12:00","2026-09-12T13:00"],"wind_speed_10m":[4.2,4.4],"wind_direction_10m":[45,50],"wind_gusts_10m":[6.4,6.8]}
}`

const seaLevelBody = `{
  "latitude":38.95,
  "longitude":-9.43,
  "utc_offset_seconds":0,
  "hourly_units":{"sea_level_height_msl":"m"},
  "hourly":{"time":["2026-09-12T12:00","2026-09-12T13:00"],"sea_level_height_msl":[0.6,0.7]}
}`

func ExampleClient() {
	config := DefaultConfig()
	fmt.Println(config.DailyLimit, config.MonthlyLimit)
	// Output: 8000 240000
}
