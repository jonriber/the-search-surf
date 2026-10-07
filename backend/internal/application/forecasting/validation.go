package forecasting

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

// ErrInvalidBatch means a provider result violated the canonical contract.
var ErrInvalidBatch = errors.New("invalid normalized forecast batch")

// ValidateBatch validates canonical values and exact request correlation.
func ValidateBatch(request FetchRequest, batch forecast.Batch, providerID string) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if batch.ProviderID != providerID || batch.ProviderID != strings.TrimSpace(batch.ProviderID) {
		return invalidBatch("provider ID does not match the selected adapter")
	}
	if !canonicalUTC(batch.FetchedAt) {
		return invalidBatch("fetch time must be a non-zero UTC instant")
	}
	if strings.TrimSpace(batch.TransformationVersion) == "" || batch.TransformationVersion != strings.TrimSpace(batch.TransformationVersion) {
		return invalidBatch("transformation version must be normalized")
	}
	if strings.TrimSpace(batch.Attribution.Text) == "" || strings.TrimSpace(batch.Attribution.License) == "" ||
		!secureURL(batch.Attribution.URL) || !secureURL(batch.Attribution.LicenseURL) {
		return invalidBatch("attribution and HTTPS licence metadata are required")
	}
	if err := validateDigests(batch.PayloadDigests); err != nil {
		return err
	}

	wantReferences := make([]string, 0, len(request.Points))
	for _, point := range request.Points {
		wantReferences = append(wantReferences, point.Reference)
	}
	slices.Sort(wantReferences)
	gotReferences := make([]string, 0, len(batch.Series))
	seenReferences := make(map[string]struct{}, len(batch.Series))
	for _, series := range batch.Series {
		if _, exists := seenReferences[series.PointReference]; exists {
			return invalidBatch("duplicate series for point %q", series.PointReference)
		}
		seenReferences[series.PointReference] = struct{}{}
		gotReferences = append(gotReferences, series.PointReference)
		if err := validateSeries(request, series); err != nil {
			return fmt.Errorf("%w: point %q: %w", ErrInvalidBatch, series.PointReference, err)
		}
	}
	slices.Sort(gotReferences)
	if !slices.Equal(gotReferences, wantReferences) {
		return invalidBatch("series references do not match requested points")
	}
	return nil
}

func validateDigests(digests []forecast.PayloadDigest) error {
	if len(digests) != 3 {
		return invalidBatch("exactly one payload digest per component is required")
	}
	seen := make(map[forecast.Component]struct{}, len(digests))
	for _, digest := range digests {
		if !validComponent(digest.Component) {
			return invalidBatch("unknown payload component %q", digest.Component)
		}
		if _, exists := seen[digest.Component]; exists {
			return invalidBatch("duplicate payload component %q", digest.Component)
		}
		seen[digest.Component] = struct{}{}
		if len(digest.SHA256) != 64 {
			return invalidBatch("payload digest must be lowercase hexadecimal SHA-256")
		}
		for _, character := range digest.SHA256 {
			if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
				return invalidBatch("payload digest must be lowercase hexadecimal SHA-256")
			}
		}
	}
	return nil
}

func validateSeries(request FetchRequest, series forecast.Series) error {
	if strings.TrimSpace(series.PointReference) == "" || series.PointReference != strings.TrimSpace(series.PointReference) {
		return errors.New("point reference must be normalized")
	}
	if len(series.Sources) != 3 {
		return errors.New("exactly one source per component is required")
	}
	seenSources := make(map[forecast.Component]struct{}, len(series.Sources))
	for _, source := range series.Sources {
		if !validComponent(source.Component) {
			return fmt.Errorf("unknown source component %q", source.Component)
		}
		if _, exists := seenSources[source.Component]; exists {
			return fmt.Errorf("duplicate source component %q", source.Component)
		}
		seenSources[source.Component] = struct{}{}
		if strings.TrimSpace(source.ModelReference) == "" || source.ModelReference != strings.TrimSpace(source.ModelReference) {
			return errors.New("source model reference must be normalized")
		}
		if !canonicalUTC(source.AvailableAt) || source.NativeTemporalResolution <= 0 {
			return errors.New("source availability and temporal resolution are required")
		}
		if source.IssueTimeKnown != !source.IssuedAt.IsZero() || source.IssueTimeKnown && (!canonicalUTC(source.IssuedAt) || source.AvailableAt.Before(source.IssuedAt)) {
			return errors.New("source issue-time semantics are inconsistent")
		}
		if invalidLongitude(source.SampledLongitude) || invalidLatitude(source.SampledLatitude) {
			return errors.New("source sampled coordinates are invalid")
		}
	}

	wantHours := int(request.EndsAt.Sub(request.StartsAt) / time.Hour)
	if len(series.Hours) != wantHours {
		return fmt.Errorf("hour count = %d, want %d", len(series.Hours), wantHours)
	}
	for index, hour := range series.Hours {
		wantTime := request.StartsAt.Add(time.Duration(index) * time.Hour)
		if !canonicalUTC(hour.ValidAt) || !hour.ValidAt.Equal(wantTime) {
			return fmt.Errorf("hour %d valid time does not match requested sequence", index)
		}
		if err := validateConditions(hour); err != nil {
			return fmt.Errorf("hour %d: %w", index, err)
		}
	}
	return nil
}

func validateConditions(hour forecast.HourlyConditions) error {
	measurements := []struct {
		name        string
		measurement forecast.Measurement
		minimum     float64
		maximum     float64
		minimumOpen bool
	}{
		{"wave height", hour.WaveHeightMetres, 0, 50, false},
		{"wave direction", hour.WaveDirectionDegrees, 0, 360, false},
		{"wave period", hour.WavePeriodSeconds, 0, 60, true},
		{"swell height", hour.SwellHeightMetres, 0, 50, false},
		{"swell direction", hour.SwellDirectionDegrees, 0, 360, false},
		{"swell period", hour.SwellPeriodSeconds, 0, 60, true},
		{"wind speed", hour.WindSpeedMetresPerSecond, 0, 150, false},
		{"wind direction", hour.WindDirectionDegrees, 0, 360, false},
		{"wind gust", hour.WindGustMetresPerSecond, 0, 150, false},
		{"sea level", hour.SeaLevelHeightMetresAboveMSL, -20, 20, false},
	}
	available := 0
	for _, candidate := range measurements {
		if !candidate.measurement.Available {
			continue
		}
		available++
		value := candidate.measurement.Value
		if math.IsNaN(value) || math.IsInf(value, 0) || value < candidate.minimum || value > candidate.maximum ||
			candidate.minimumOpen && value == candidate.minimum ||
			(candidate.name == "wave direction" || candidate.name == "swell direction" || candidate.name == "wind direction") && value == candidate.maximum {
			return fmt.Errorf("%s is outside its plausible canonical range", candidate.name)
		}
	}
	if available == 0 {
		return errors.New("at least one measurement is required")
	}
	return nil
}

func canonicalUTC(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC
}

func secureURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func validComponent(component forecast.Component) bool {
	return component == forecast.ComponentWaves || component == forecast.ComponentWind || component == forecast.ComponentSeaLevel
}

func invalidLongitude(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0) || value < -180 || value > 180
}

func invalidLatitude(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0) || value < -90 || value > 90
}

func invalidBatch(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidBatch, fmt.Sprintf(format, arguments...))
}
