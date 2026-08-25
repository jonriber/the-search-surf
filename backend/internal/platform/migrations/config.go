package migrations

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultTimeout = 2 * time.Minute

// LookupEnv matches os.LookupEnv and keeps configuration tests process-independent.
type LookupEnv func(key string) (string, bool)

// Config contains validated migration-process configuration.
type Config struct {
	DatabaseURL string
	Timeout     time.Duration
}

// ConfigFromEnvironment loads migration configuration without exposing credential values in errors.
func ConfigFromEnvironment(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("environment lookup function is required")
	}

	databaseURL, ok := lookup("MIGRATION_DATABASE_URL")
	if !ok || strings.TrimSpace(databaseURL) == "" {
		return Config{}, errors.New("MIGRATION_DATABASE_URL is required")
	}

	timeout := defaultTimeout
	if value, configured := lookup("MIGRATION_TIMEOUT"); configured {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("MIGRATION_TIMEOUT: parse duration: %w", err)
		}
		if parsed <= 0 {
			return Config{}, errors.New("MIGRATION_TIMEOUT must be positive")
		}
		timeout = parsed
	}

	return Config{DatabaseURL: databaseURL, Timeout: timeout}, nil
}
