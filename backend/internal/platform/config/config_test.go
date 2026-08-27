package config

import (
	"testing"
	"time"
)

var requiredValues = map[string]string{
	"DATABASE_URL":           "postgres://the_search_app@database:5432/the_search?sslmode=disable",
	"BOOTSTRAP_PRINCIPAL_ID": "2f404f62-3d6f-4e5f-a2e8-1be44b08f05c",
}

func lookupWith(overrides map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		if value, ok := overrides[key]; ok {
			return value, true
		}
		value, ok := requiredValues[key]
		return value, ok
	}
}

func TestFromEnvironmentDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := FromEnvironment(lookupWith(nil))
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}

	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":8080")
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want %s", cfg.ShutdownTimeout, 10*time.Second)
	}
	if cfg.DatabaseURL != requiredValues["DATABASE_URL"] || cfg.PrincipalID.String() != requiredValues["BOOTSTRAP_PRINCIPAL_ID"] {
		t.Fatalf("database identity configuration = (%q, %s)", cfg.DatabaseURL, cfg.PrincipalID)
	}
	if cfg.DatabaseConnectTimeout != 5*time.Second {
		t.Fatalf("DatabaseConnectTimeout = %s, want 5s", cfg.DatabaseConnectTimeout)
	}
}

func TestFromEnvironmentOverridesValues(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"HTTP_ADDRESS":             "127.0.0.1:9000",
		"HTTP_READ_TIMEOUT":        "2s",
		"HTTP_WRITE_TIMEOUT":       "3s",
		"HTTP_IDLE_TIMEOUT":        "4s",
		"HTTP_SHUTDOWN_TIMEOUT":    "5s",
		"DATABASE_CONNECT_TIMEOUT": "6s",
	}

	cfg, err := FromEnvironment(lookupWith(values))
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}

	if cfg.HTTPAddress != "127.0.0.1:9000" || cfg.ReadTimeout != 2*time.Second || cfg.WriteTimeout != 3*time.Second || cfg.IdleTimeout != 4*time.Second || cfg.ShutdownTimeout != 5*time.Second || cfg.DatabaseConnectTimeout != 6*time.Second {
		t.Fatalf("FromEnvironment() = %+v, want configured values", cfg)
	}
}

func TestFromEnvironmentRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "empty address", key: "HTTP_ADDRESS", value: ""},
		{name: "missing port", key: "HTTP_ADDRESS", value: "localhost"},
		{name: "invalid duration", key: "HTTP_READ_TIMEOUT", value: "soon"},
		{name: "non-positive duration", key: "HTTP_SHUTDOWN_TIMEOUT", value: "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := FromEnvironment(lookupWith(map[string]string{tt.key: tt.value}))
			if err == nil {
				t.Fatal("FromEnvironment() error = nil, want validation error")
			}
		})
	}
}

func TestFromEnvironmentRequiresDatabaseAndPrincipal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]string
	}{
		{name: "database missing", values: map[string]string{"BOOTSTRAP_PRINCIPAL_ID": requiredValues["BOOTSTRAP_PRINCIPAL_ID"]}},
		{name: "database empty", values: map[string]string{"DATABASE_URL": "", "BOOTSTRAP_PRINCIPAL_ID": requiredValues["BOOTSTRAP_PRINCIPAL_ID"]}},
		{name: "principal missing", values: map[string]string{"DATABASE_URL": requiredValues["DATABASE_URL"]}},
		{name: "principal invalid", values: map[string]string{"DATABASE_URL": requiredValues["DATABASE_URL"], "BOOTSTRAP_PRINCIPAL_ID": "not-a-uuid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := FromEnvironment(func(key string) (string, bool) {
				value, ok := tt.values[key]
				return value, ok
			})
			if err == nil {
				t.Fatal("FromEnvironment() error = nil")
			}
		})
	}
}

func TestFromEnvironmentRequiresLookup(t *testing.T) {
	t.Parallel()

	if _, err := FromEnvironment(nil); err == nil {
		t.Fatal("FromEnvironment(nil) error = nil, want error")
	}
}
