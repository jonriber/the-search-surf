// Package forecastworker owns configuration and orchestration for the one-shot
// forecast ingestion process.
package forecastworker

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/platform/openmeteo"
)

const (
	defaultHorizon = 72 * time.Hour
	maximumHorizon = 16 * 24 * time.Hour
	defaultTimeout = 30 * time.Second
)

// LookupEnv matches os.LookupEnv for deterministic configuration tests.
type LookupEnv func(string) (string, bool)

// Config is the validated worker runtime configuration.
type Config struct {
	DatabaseURL string
	Horizon     time.Duration
	Timeout     time.Duration
	OpenMeteo   openmeteo.Config
}

// ConfigFromEnvironment loads worker and provider policy from the environment.
func ConfigFromEnvironment(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("environment lookup function is required")
	}
	databaseURL, ok := lookup("FORECAST_DATABASE_URL")
	if !ok || strings.TrimSpace(databaseURL) == "" {
		return Config{}, errors.New("FORECAST_DATABASE_URL is required")
	}
	horizon, err := durationValue(lookup, "FORECAST_HORIZON", defaultHorizon)
	if err != nil {
		return Config{}, err
	}
	if horizon <= 0 || horizon > maximumHorizon || horizon%time.Hour != 0 {
		return Config{}, errors.New("FORECAST_HORIZON must be a positive whole-hour duration no greater than 384h")
	}
	timeout, err := durationValue(lookup, "FORECAST_TIMEOUT", defaultTimeout)
	if err != nil {
		return Config{}, err
	}
	if timeout <= 0 {
		return Config{}, errors.New("FORECAST_TIMEOUT must be positive")
	}

	providerConfig := openmeteo.DefaultConfig()
	providerConfig.AccountScope = stringValue(lookup, "OPEN_METEO_ACCOUNT_SCOPE", providerConfig.AccountScope)
	providerConfig.DailyLimit, err = integerValue(lookup, "OPEN_METEO_DAILY_LIMIT", providerConfig.DailyLimit)
	if err != nil {
		return Config{}, err
	}
	providerConfig.MonthlyLimit, err = integerValue(lookup, "OPEN_METEO_MONTHLY_LIMIT", providerConfig.MonthlyLimit)
	if err != nil {
		return Config{}, err
	}
	providerConfig.ForecastURL = stringValue(lookup, "OPEN_METEO_FORECAST_URL", providerConfig.ForecastURL)
	providerConfig.MarineURL = stringValue(lookup, "OPEN_METEO_MARINE_URL", providerConfig.MarineURL)
	providerConfig.WaveMetadataURL = stringValue(lookup, "OPEN_METEO_WAVE_METADATA_URL", providerConfig.WaveMetadataURL)
	providerConfig.WindMetadataURL = stringValue(lookup, "OPEN_METEO_WIND_METADATA_URL", providerConfig.WindMetadataURL)
	providerConfig.SeaLevelMetadataURL = stringValue(lookup, "OPEN_METEO_SEA_LEVEL_METADATA_URL", providerConfig.SeaLevelMetadataURL)

	return Config{DatabaseURL: databaseURL, Horizon: horizon, Timeout: timeout, OpenMeteo: providerConfig}, nil
}

func durationValue(lookup LookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	value, exists := lookup(key)
	if !exists {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: parse duration: %w", key, err)
	}
	return parsed, nil
}

func integerValue(lookup LookupEnv, key string, fallback int64) (int64, error) {
	value, exists := lookup(key)
	if !exists {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}

func stringValue(lookup LookupEnv, key, fallback string) string {
	if value, exists := lookup(key); exists && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
