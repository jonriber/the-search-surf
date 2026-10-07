package openmeteo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

type componentSpec struct {
	component   forecast.Component
	model       string
	variables   []string
	dataURL     func(Config) string
	metadataURL func(Config) string
}

var componentSpecs = []componentSpec{
	{
		component: forecast.ComponentWaves,
		model:     "meteofrance_wave",
		variables: []string{
			"wave_height", "wave_direction", "wave_period",
			"swell_wave_height", "swell_wave_direction", "swell_wave_period",
		},
		dataURL:     func(config Config) string { return config.MarineURL },
		metadataURL: func(config Config) string { return config.WaveMetadataURL },
	},
	{
		component:   forecast.ComponentWind,
		model:       "ecmwf_ifs",
		variables:   []string{"wind_speed_10m", "wind_direction_10m", "wind_gusts_10m"},
		dataURL:     func(config Config) string { return config.ForecastURL },
		metadataURL: func(config Config) string { return config.WindMetadataURL },
	},
	{
		component:   forecast.ComponentSeaLevel,
		model:       "meteofrance_currents",
		variables:   []string{"sea_level_height_msl"},
		dataURL:     func(config Config) string { return config.MarineURL },
		metadataURL: func(config Config) string { return config.SeaLevelMetadataURL },
	},
}

type modelMetadata struct {
	issuedAt           time.Time
	availableAt        time.Time
	temporalResolution time.Duration
}

func decodeMetadata(body []byte) (modelMetadata, error) {
	var payload struct {
		LastRunInitialisationTime int64 `json:"last_run_initialisation_time"`
		LastRunAvailabilityTime   int64 `json:"last_run_availability_time"`
		TemporalResolutionSeconds int64 `json:"temporal_resolution_seconds"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err != nil {
		return modelMetadata{}, fmt.Errorf("decode model metadata: %w", err)
	}
	if payload.LastRunInitialisationTime <= 0 || payload.LastRunAvailabilityTime <= 0 || payload.TemporalResolutionSeconds <= 0 {
		return modelMetadata{}, errors.New("model metadata contains non-positive required values")
	}
	issuedAt := time.Unix(payload.LastRunInitialisationTime, 0).UTC()
	availableAt := time.Unix(payload.LastRunAvailabilityTime, 0).UTC()
	if availableAt.Before(issuedAt) {
		return modelMetadata{}, errors.New("model availability precedes initialization")
	}
	return modelMetadata{
		issuedAt:           issuedAt,
		availableAt:        availableAt,
		temporalResolution: time.Duration(payload.TemporalResolutionSeconds) * time.Second,
	}, nil
}

type apiResponse struct {
	Latitude         float64           `json:"latitude"`
	Longitude        float64           `json:"longitude"`
	UTCOffsetSeconds int64             `json:"utc_offset_seconds"`
	HourlyUnits      map[string]string `json:"hourly_units"`
	Hourly           apiHourly         `json:"hourly"`
}

type apiHourly struct {
	Time              []string   `json:"time"`
	WaveHeight        []*float64 `json:"wave_height"`
	WaveDirection     []*float64 `json:"wave_direction"`
	WavePeriod        []*float64 `json:"wave_period"`
	SwellHeight       []*float64 `json:"swell_wave_height"`
	SwellDirection    []*float64 `json:"swell_wave_direction"`
	SwellPeriod       []*float64 `json:"swell_wave_period"`
	WindSpeed         []*float64 `json:"wind_speed_10m"`
	WindDirection     []*float64 `json:"wind_direction_10m"`
	WindGust          []*float64 `json:"wind_gusts_10m"`
	SeaLevelHeightMSL []*float64 `json:"sea_level_height_msl"`
}

func initializeSeries(request forecasting.FetchRequest) []forecast.Series {
	hourCount := int(request.EndsAt.Sub(request.StartsAt) / time.Hour)
	series := make([]forecast.Series, 0, len(request.Points))
	for _, point := range request.Points {
		hours := make([]forecast.HourlyConditions, hourCount)
		for index := range hours {
			hours[index].ValidAt = request.StartsAt.Add(time.Duration(index) * time.Hour)
		}
		series = append(series, forecast.Series{PointReference: point.Reference, Hours: hours})
	}
	return series
}

func decodeComponent(
	body []byte,
	request forecasting.FetchRequest,
	spec componentSpec,
	metadata modelMetadata,
	series []forecast.Series,
) error {
	responses, err := decodeResponses(body)
	if err != nil {
		return err
	}
	if len(responses) != len(request.Points) {
		return fmt.Errorf("response location count = %d, want %d", len(responses), len(request.Points))
	}
	for pointIndex, response := range responses {
		if response.UTCOffsetSeconds != 0 {
			return fmt.Errorf("location %d is not UTC", pointIndex)
		}
		if invalidCoordinate(response.Longitude, -180, 180) || invalidCoordinate(response.Latitude, -90, 90) {
			return fmt.Errorf("location %d has invalid sampled coordinates", pointIndex)
		}
		if err := validateUnits(response.HourlyUnits, spec); err != nil {
			return fmt.Errorf("location %d: %w", pointIndex, err)
		}
		if err := applyHourly(response.Hourly, request, spec.component, &series[pointIndex]); err != nil {
			return fmt.Errorf("location %d: %w", pointIndex, err)
		}
		series[pointIndex].Sources = append(series[pointIndex].Sources, forecast.Source{
			Component:                spec.component,
			ModelReference:           spec.model,
			IssuedAt:                 metadata.issuedAt,
			AvailableAt:              metadata.availableAt,
			IssueTimeKnown:           true,
			SampledLongitude:         response.Longitude,
			SampledLatitude:          response.Latitude,
			NativeTemporalResolution: metadata.temporalResolution,
		})
	}
	return nil
}

func decodeResponses(body []byte) ([]apiResponse, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errors.New("provider returned an empty body")
	}
	if trimmed[0] == '[' {
		var responses []apiResponse
		if err := json.Unmarshal(trimmed, &responses); err != nil {
			return nil, fmt.Errorf("decode provider locations: %w", err)
		}
		return responses, nil
	}
	var response apiResponse
	if err := json.Unmarshal(trimmed, &response); err != nil {
		return nil, fmt.Errorf("decode provider location: %w", err)
	}
	return []apiResponse{response}, nil
}

func validateUnits(units map[string]string, spec componentSpec) error {
	var want map[string]string
	switch spec.component {
	case forecast.ComponentWaves:
		want = map[string]string{
			"wave_height": "m", "wave_direction": "°", "wave_period": "s",
			"swell_wave_height": "m", "swell_wave_direction": "°", "swell_wave_period": "s",
		}
	case forecast.ComponentWind:
		want = map[string]string{
			"wind_speed_10m": "m/s", "wind_direction_10m": "°", "wind_gusts_10m": "m/s",
		}
	case forecast.ComponentSeaLevel:
		want = map[string]string{"sea_level_height_msl": "m"}
	default:
		return fmt.Errorf("unknown component %q", spec.component)
	}
	for variable, unit := range want {
		if units[variable] != unit {
			return fmt.Errorf("unit for %s = %q, want %q", variable, units[variable], unit)
		}
	}
	return nil
}

func applyHourly(hourly apiHourly, request forecasting.FetchRequest, component forecast.Component, series *forecast.Series) error {
	arrays := componentArrays(hourly, component)
	if len(hourly.Time) == 0 {
		return errors.New("hourly time array is empty")
	}
	for name, values := range arrays {
		if len(values) != len(hourly.Time) {
			return fmt.Errorf("%s array length = %d, time length = %d", name, len(values), len(hourly.Time))
		}
	}
	seen := make(map[time.Time]struct{}, len(hourly.Time))
	matched := 0
	for index, rawTime := range hourly.Time {
		validAt, err := parseProviderTime(rawTime)
		if err != nil {
			return fmt.Errorf("parse hourly time %q: %w", rawTime, err)
		}
		if _, exists := seen[validAt]; exists {
			return fmt.Errorf("duplicate hourly time %s", validAt.Format(time.RFC3339))
		}
		seen[validAt] = struct{}{}
		if validAt.Before(request.StartsAt) || !validAt.Before(request.EndsAt) {
			continue
		}
		targetIndex := int(validAt.Sub(request.StartsAt) / time.Hour)
		if !validAt.Equal(request.StartsAt.Add(time.Duration(targetIndex) * time.Hour)) {
			return fmt.Errorf("hourly time %s is not aligned", validAt.Format(time.RFC3339))
		}
		if err := setMeasurements(component, hourly, index, &series.Hours[targetIndex]); err != nil {
			return err
		}
		matched++
	}
	if matched != len(series.Hours) {
		return fmt.Errorf("matched hour count = %d, want %d", matched, len(series.Hours))
	}
	return nil
}

func componentArrays(hourly apiHourly, component forecast.Component) map[string][]*float64 {
	switch component {
	case forecast.ComponentWaves:
		return map[string][]*float64{
			"wave_height": hourly.WaveHeight, "wave_direction": hourly.WaveDirection,
			"wave_period": hourly.WavePeriod, "swell_wave_height": hourly.SwellHeight,
			"swell_wave_direction": hourly.SwellDirection, "swell_wave_period": hourly.SwellPeriod,
		}
	case forecast.ComponentWind:
		return map[string][]*float64{
			"wind_speed_10m": hourly.WindSpeed, "wind_direction_10m": hourly.WindDirection,
			"wind_gusts_10m": hourly.WindGust,
		}
	case forecast.ComponentSeaLevel:
		return map[string][]*float64{"sea_level_height_msl": hourly.SeaLevelHeightMSL}
	default:
		return nil
	}
}

func setMeasurements(component forecast.Component, hourly apiHourly, index int, target *forecast.HourlyConditions) error {
	var candidates []struct {
		name   string
		value  *float64
		assign func(forecast.Measurement)
	}
	switch component {
	case forecast.ComponentWaves:
		candidates = []struct {
			name   string
			value  *float64
			assign func(forecast.Measurement)
		}{
			{"wave_height", hourly.WaveHeight[index], func(value forecast.Measurement) { target.WaveHeightMetres = value }},
			{"wave_direction", hourly.WaveDirection[index], func(value forecast.Measurement) { target.WaveDirectionDegrees = value }},
			{"wave_period", hourly.WavePeriod[index], func(value forecast.Measurement) { target.WavePeriodSeconds = value }},
			{"swell_wave_height", hourly.SwellHeight[index], func(value forecast.Measurement) { target.SwellHeightMetres = value }},
			{"swell_wave_direction", hourly.SwellDirection[index], func(value forecast.Measurement) { target.SwellDirectionDegrees = value }},
			{"swell_wave_period", hourly.SwellPeriod[index], func(value forecast.Measurement) { target.SwellPeriodSeconds = value }},
		}
	case forecast.ComponentWind:
		candidates = []struct {
			name   string
			value  *float64
			assign func(forecast.Measurement)
		}{
			{"wind_speed_10m", hourly.WindSpeed[index], func(value forecast.Measurement) { target.WindSpeedMetresPerSecond = value }},
			{"wind_direction_10m", hourly.WindDirection[index], func(value forecast.Measurement) { target.WindDirectionDegrees = value }},
			{"wind_gusts_10m", hourly.WindGust[index], func(value forecast.Measurement) { target.WindGustMetresPerSecond = value }},
		}
	case forecast.ComponentSeaLevel:
		candidates = []struct {
			name   string
			value  *float64
			assign func(forecast.Measurement)
		}{
			{"sea_level_height_msl", hourly.SeaLevelHeightMSL[index], func(value forecast.Measurement) { target.SeaLevelHeightMetresAboveMSL = value }},
		}
	default:
		return fmt.Errorf("unknown component %q", component)
	}
	for _, candidate := range candidates {
		if candidate.value == nil {
			candidate.assign(forecast.MissingMeasurement())
			continue
		}
		measurement, err := forecast.NewMeasurement(*candidate.value)
		if err != nil {
			return fmt.Errorf("%s: %w", candidate.name, err)
		}
		if !plausibleProviderValue(candidate.name, measurement.Value) {
			return fmt.Errorf("%s is outside its plausible canonical range", candidate.name)
		}
		candidate.assign(measurement)
	}
	return nil
}

func plausibleProviderValue(name string, value float64) bool {
	switch name {
	case "wave_height", "swell_wave_height":
		return value >= 0 && value <= 50
	case "wave_direction", "swell_wave_direction", "wind_direction_10m":
		return value >= 0 && value < 360
	case "wave_period", "swell_wave_period":
		return value > 0 && value <= 60
	case "wind_speed_10m", "wind_gusts_10m":
		return value >= 0 && value <= 150
	case "sea_level_height_msl":
		return value >= -20 && value <= 20
	default:
		return false
	}
}

func parseProviderTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, errors.New("unsupported provider timestamp")
}

func invalidCoordinate(value, minimum, maximum float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0) || value < minimum || value > maximum
}
