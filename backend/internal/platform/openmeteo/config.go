// Package openmeteo implements the provider-neutral forecast port using
// explicit Open-Meteo models and metadata-bracketed reads.
package openmeteo

import (
	"errors"
	"time"
)

const (
	// ProviderID is the stable application identity for this adapter.
	ProviderID = "open-meteo"
	// TransformationVersion identifies these field mappings and validations.
	TransformationVersion = "open-meteo-v1"
)

// Config contains endpoint, quota, and bounded-resilience policy.
type Config struct {
	ForecastURL         string
	MarineURL           string
	WaveMetadataURL     string
	WindMetadataURL     string
	SeaLevelMetadataURL string
	AccountScope        string
	DailyLimit          int64
	MonthlyLimit        int64
	PhysicalTimeout     time.Duration
	LogicalTimeout      time.Duration
	MaximumRetries      int
	InitialBackoff      time.Duration
	MaximumBackoff      time.Duration
	AvailabilityLag     time.Duration
	MaximumResponseSize int64
}

// DefaultConfig returns the ADR-approved non-commercial configuration.
func DefaultConfig() Config {
	return Config{
		ForecastURL:         "https://api.open-meteo.com/v1/forecast",
		MarineURL:           "https://marine-api.open-meteo.com/v1/marine",
		WaveMetadataURL:     "https://marine-api.open-meteo.com/data/meteofrance_wave/static/meta.json",
		WindMetadataURL:     "https://api.open-meteo.com/data/ecmwf_ifs/static/meta.json",
		SeaLevelMetadataURL: "https://marine-api.open-meteo.com/data/meteofrance_currents/static/meta.json",
		AccountScope:        "non-commercial-free",
		DailyLimit:          8_000,
		MonthlyLimit:        240_000,
		PhysicalTimeout:     5 * time.Second,
		LogicalTimeout:      15 * time.Second,
		MaximumRetries:      2,
		InitialBackoff:      250 * time.Millisecond,
		MaximumBackoff:      2 * time.Second,
		AvailabilityLag:     10 * time.Minute,
		MaximumResponseSize: 4 << 20,
	}
}

func (config Config) validate() error {
	if config.ForecastURL == "" || config.MarineURL == "" || config.WaveMetadataURL == "" ||
		config.WindMetadataURL == "" || config.SeaLevelMetadataURL == "" {
		return errors.New("Open-Meteo endpoint URLs are required")
	}
	if config.AccountScope == "" || config.DailyLimit < 1 || config.MonthlyLimit < 1 {
		return errors.New("Open-Meteo account scope and positive quota limits are required")
	}
	if config.PhysicalTimeout <= 0 || config.LogicalTimeout <= 0 || config.MaximumRetries < 0 ||
		config.InitialBackoff < 0 || config.MaximumBackoff < config.InitialBackoff ||
		config.AvailabilityLag < 0 || config.MaximumResponseSize < 1 {
		return errors.New("Open-Meteo resilience limits are invalid")
	}
	return nil
}
