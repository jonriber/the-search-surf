// Package config loads and validates process configuration at the application boundary.
package config

import (
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	defaultHTTPAddress     = ":8080"
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

// LookupEnv matches os.LookupEnv and keeps configuration tests independent from process state.
type LookupEnv func(key string) (string, bool)

// Config contains validated runtime settings for the API process.
type Config struct {
	HTTPAddress     string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// FromEnvironment loads configuration with safe defaults and rejects invalid explicit values.
func FromEnvironment(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("environment lookup function is required")
	}

	cfg := Config{
		HTTPAddress:     valueOrDefault(lookup, "HTTP_ADDRESS", defaultHTTPAddress),
		ReadTimeout:     defaultReadTimeout,
		WriteTimeout:    defaultWriteTimeout,
		IdleTimeout:     defaultIdleTimeout,
		ShutdownTimeout: defaultShutdownTimeout,
	}

	if err := validateAddress(cfg.HTTPAddress); err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDRESS: %w", err)
	}

	var err error
	if cfg.ReadTimeout, err = durationOrDefault(lookup, "HTTP_READ_TIMEOUT", defaultReadTimeout); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = durationOrDefault(lookup, "HTTP_WRITE_TIMEOUT", defaultWriteTimeout); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = durationOrDefault(lookup, "HTTP_IDLE_TIMEOUT", defaultIdleTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationOrDefault(lookup, "HTTP_SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func valueOrDefault(lookup LookupEnv, key, fallback string) string {
	if value, ok := lookup(key); ok {
		return value
	}
	return fallback
}

func durationOrDefault(lookup LookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	value, ok := lookup(key)
	if !ok {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: parse duration: %w", key, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s: duration must be positive", key)
	}

	return duration, nil
}

func validateAddress(address string) error {
	if address == "" {
		return errors.New("address must not be empty")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return fmt.Errorf("invalid host and port: %w", err)
	}
	return nil
}
