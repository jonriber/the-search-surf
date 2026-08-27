// Package bootstrapprincipal owns the one-shot bootstrap identity process.
package bootstrapprincipal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

const defaultTimeout = 30 * time.Second

// LookupEnv matches os.LookupEnv for deterministic configuration tests.
type LookupEnv func(string) (string, bool)

// Config is the validated bootstrap command configuration.
type Config struct {
	DatabaseURL string
	PrincipalID identity.PrincipalID
	Timeout     time.Duration
}

// ConfigFromEnvironment loads explicit bootstrap identity configuration.
func ConfigFromEnvironment(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("environment lookup function is required")
	}

	databaseURL, ok := lookup("BOOTSTRAP_DATABASE_URL")
	if !ok || strings.TrimSpace(databaseURL) == "" {
		return Config{}, errors.New("BOOTSTRAP_DATABASE_URL is required")
	}
	principalValue, ok := lookup("BOOTSTRAP_PRINCIPAL_ID")
	if !ok || strings.TrimSpace(principalValue) == "" {
		return Config{}, errors.New("BOOTSTRAP_PRINCIPAL_ID is required")
	}
	principalID, err := identity.ParsePrincipalID(principalValue)
	if err != nil {
		return Config{}, fmt.Errorf("BOOTSTRAP_PRINCIPAL_ID: %w", err)
	}

	timeout := defaultTimeout
	if value, exists := lookup("BOOTSTRAP_TIMEOUT"); exists {
		timeout, err = time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("BOOTSTRAP_TIMEOUT: parse duration: %w", err)
		}
		if timeout <= 0 {
			return Config{}, errors.New("BOOTSTRAP_TIMEOUT must be positive")
		}
	}

	return Config{DatabaseURL: databaseURL, PrincipalID: principalID, Timeout: timeout}, nil
}
