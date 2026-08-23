package config

import (
	"testing"
	"time"
)

func TestFromEnvironmentDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := FromEnvironment(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}

	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":8080")
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want %s", cfg.ShutdownTimeout, 10*time.Second)
	}
}

func TestFromEnvironmentOverridesValues(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"HTTP_ADDRESS":          "127.0.0.1:9000",
		"HTTP_READ_TIMEOUT":     "2s",
		"HTTP_WRITE_TIMEOUT":    "3s",
		"HTTP_IDLE_TIMEOUT":     "4s",
		"HTTP_SHUTDOWN_TIMEOUT": "5s",
	}

	cfg, err := FromEnvironment(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}

	if cfg.HTTPAddress != "127.0.0.1:9000" || cfg.ReadTimeout != 2*time.Second || cfg.WriteTimeout != 3*time.Second || cfg.IdleTimeout != 4*time.Second || cfg.ShutdownTimeout != 5*time.Second {
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
			_, err := FromEnvironment(func(key string) (string, bool) {
				if key == tt.key {
					return tt.value, true
				}
				return "", false
			})
			if err == nil {
				t.Fatal("FromEnvironment() error = nil, want validation error")
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
