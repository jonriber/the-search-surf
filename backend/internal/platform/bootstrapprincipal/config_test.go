package bootstrapprincipal

import (
	"strings"
	"testing"
	"time"
)

func TestConfigFromEnvironment(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"BOOTSTRAP_DATABASE_URL": "postgres://bootstrap@database/the_search",
		"BOOTSTRAP_PRINCIPAL_ID": "2f404f62-3d6f-4e5f-a2e8-1be44b08f05c",
		"BOOTSTRAP_TIMEOUT":      "45s",
	}

	cfg, err := ConfigFromEnvironment(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("ConfigFromEnvironment() error = %v", err)
	}
	if cfg.DatabaseURL != values["BOOTSTRAP_DATABASE_URL"] || cfg.PrincipalID.String() != values["BOOTSTRAP_PRINCIPAL_ID"] || cfg.Timeout != 45*time.Second {
		t.Fatalf("ConfigFromEnvironment() = %+v", cfg)
	}
}

func TestConfigFromEnvironmentUsesSafeTimeoutDefault(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"BOOTSTRAP_DATABASE_URL": "postgres://bootstrap@database/the_search",
		"BOOTSTRAP_PRINCIPAL_ID": "2f404f62-3d6f-4e5f-a2e8-1be44b08f05c",
	}
	cfg, err := ConfigFromEnvironment(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("ConfigFromEnvironment() error = %v", err)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %s, want 30s", cfg.Timeout)
	}
}

func TestConfigFromEnvironmentRejectsMissingOrInvalidValuesWithoutLeakingDSN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]string
	}{
		{name: "missing database URL", values: map[string]string{"BOOTSTRAP_PRINCIPAL_ID": "2f404f62-3d6f-4e5f-a2e8-1be44b08f05c"}},
		{name: "missing principal", values: map[string]string{"BOOTSTRAP_DATABASE_URL": "postgres://user@database/db"}},
		{name: "invalid principal", values: map[string]string{"BOOTSTRAP_DATABASE_URL": "postgres://user@database/db", "BOOTSTRAP_PRINCIPAL_ID": "invalid"}},
		{name: "invalid timeout", values: map[string]string{"BOOTSTRAP_DATABASE_URL": "postgres://user@database/db", "BOOTSTRAP_PRINCIPAL_ID": "2f404f62-3d6f-4e5f-a2e8-1be44b08f05c", "BOOTSTRAP_TIMEOUT": "never"}},
		{name: "non-positive timeout", values: map[string]string{"BOOTSTRAP_DATABASE_URL": "postgres://user@database/db", "BOOTSTRAP_PRINCIPAL_ID": "2f404f62-3d6f-4e5f-a2e8-1be44b08f05c", "BOOTSTRAP_TIMEOUT": "0s"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ConfigFromEnvironment(func(key string) (string, bool) {
				value, ok := tt.values[key]
				return value, ok
			})
			if err == nil {
				t.Fatal("ConfigFromEnvironment() error = nil")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaks database URL: %v", err)
			}
		})
	}
}

func TestConfigFromEnvironmentRequiresLookup(t *testing.T) {
	t.Parallel()

	if _, err := ConfigFromEnvironment(nil); err == nil {
		t.Fatal("ConfigFromEnvironment(nil) error = nil")
	}
}
