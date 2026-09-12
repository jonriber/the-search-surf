package forecasting

import (
	"errors"
	"testing"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

func TestValidateBatchAcceptsCompleteCanonicalBatch(t *testing.T) {
	request := validRequest(t)
	if err := ValidateBatch(request, validBatch(request), "open-meteo"); err != nil {
		t.Fatalf("ValidateBatch() error = %v", err)
	}
}

func TestValidateBatchRejectsBoundaryAndCorrelationFailures(t *testing.T) {
	request := validRequest(t)
	tests := []struct {
		name   string
		mutate func(*forecast.Batch)
	}{
		{name: "provider mismatch", mutate: func(batch *forecast.Batch) { batch.ProviderID = "another" }},
		{name: "non UTC fetch time", mutate: func(batch *forecast.Batch) { batch.FetchedAt = batch.FetchedAt.In(time.FixedZone("WET", 0)) }},
		{name: "duplicate digest component", mutate: func(batch *forecast.Batch) { batch.PayloadDigests[1].Component = forecast.ComponentWaves }},
		{name: "bad digest", mutate: func(batch *forecast.Batch) { batch.PayloadDigests[0].SHA256 = "ABC" }},
		{name: "unknown series", mutate: func(batch *forecast.Batch) { batch.Series[0].PointReference = "unknown" }},
		{name: "missing source", mutate: func(batch *forecast.Batch) { batch.Series[0].Sources = batch.Series[0].Sources[:2] }},
		{name: "inconsistent issue time", mutate: func(batch *forecast.Batch) { batch.Series[0].Sources[0].IssueTimeKnown = false }},
		{name: "wrong hourly sequence", mutate: func(batch *forecast.Batch) { batch.Series[0].Hours[0].ValidAt = request.StartsAt.Add(time.Hour) }},
		{name: "implausible wave", mutate: func(batch *forecast.Batch) { batch.Series[0].Hours[0].WaveHeightMetres = available(51) }},
		{name: "direction upper bound", mutate: func(batch *forecast.Batch) { batch.Series[0].Hours[0].WindDirectionDegrees = available(360) }},
		{name: "empty observation", mutate: func(batch *forecast.Batch) {
			batch.Series[0].Hours[0] = forecast.HourlyConditions{ValidAt: request.StartsAt}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			batch := validBatch(request)
			test.mutate(&batch)
			if err := ValidateBatch(request, batch, "open-meteo"); !errors.Is(err, ErrInvalidBatch) {
				t.Fatalf("ValidateBatch() error = %v, want ErrInvalidBatch", err)
			}
		})
	}
}

func validRequest(t *testing.T) FetchRequest {
	t.Helper()
	point, err := forecast.NewPoint("11111111-1111-4111-8111-111111111111", -9.417, 38.963)
	if err != nil {
		t.Fatal(err)
	}
	request, err := NewFetchRequest(
		[]forecast.Point{point},
		time.Date(2026, time.September, 11, 12, 0, 0, 0, time.UTC),
		time.Date(2026, time.September, 11, 14, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func validBatch(request FetchRequest) forecast.Batch {
	issuedAt := request.StartsAt.Add(-6 * time.Hour)
	availableAt := issuedAt.Add(90 * time.Minute)
	sources := make([]forecast.Source, 0, 3)
	for _, component := range []forecast.Component{forecast.ComponentWaves, forecast.ComponentWind, forecast.ComponentSeaLevel} {
		sources = append(sources, forecast.Source{
			Component:                component,
			ModelReference:           "explicit-model",
			IssuedAt:                 issuedAt,
			AvailableAt:              availableAt,
			IssueTimeKnown:           true,
			SampledLongitude:         -9.42,
			SampledLatitude:          38.96,
			NativeTemporalResolution: time.Hour,
		})
	}
	hours := []forecast.HourlyConditions{
		validHour(request.StartsAt),
		validHour(request.StartsAt.Add(time.Hour)),
	}
	return forecast.Batch{
		ProviderID:            "open-meteo",
		FetchedAt:             request.StartsAt.Add(5 * time.Minute),
		TransformationVersion: "open-meteo-v1",
		Attribution: forecast.Attribution{
			Text:       "Weather data by Open-Meteo.com",
			URL:        "https://open-meteo.com/",
			License:    "CC BY 4.0",
			LicenseURL: "https://creativecommons.org/licenses/by/4.0/",
		},
		PayloadDigests: []forecast.PayloadDigest{
			{Component: forecast.ComponentWaves, SHA256: repeatedHex('a')},
			{Component: forecast.ComponentWind, SHA256: repeatedHex('b')},
			{Component: forecast.ComponentSeaLevel, SHA256: repeatedHex('c')},
		},
		Series: []forecast.Series{{
			PointReference: request.Points[0].Reference,
			Sources:        sources,
			Hours:          hours,
		}},
	}
}

func validHour(validAt time.Time) forecast.HourlyConditions {
	return forecast.HourlyConditions{
		ValidAt:                      validAt,
		WaveHeightMetres:             available(1.7),
		WaveDirectionDegrees:         available(287),
		WavePeriodSeconds:            available(11),
		SwellHeightMetres:            available(1.3),
		SwellDirectionDegrees:        available(292),
		SwellPeriodSeconds:           available(13),
		WindSpeedMetresPerSecond:     available(4.2),
		WindDirectionDegrees:         available(45),
		WindGustMetresPerSecond:      available(6.4),
		SeaLevelHeightMetresAboveMSL: available(0.6),
	}
}

func available(value float64) forecast.Measurement {
	return forecast.Measurement{Value: value, Available: true}
}

func repeatedHex(character byte) string {
	value := make([]byte, 64)
	for index := range value {
		value[index] = character
	}
	return string(value)
}
