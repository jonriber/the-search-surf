package migrations_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/platform/migrations"
)

func TestConfigFromEnvironment(t *testing.T) {
	t.Parallel()
	credentialMarker := strings.Repeat("s", 6)

	tests := []struct {
		name        string
		environment map[string]string
		wantTimeout time.Duration
		wantErr     string
	}{
		{
			name:        "safe default timeout",
			environment: map[string]string{"MIGRATION_DATABASE_URL": "postgres://database.example/the_search"},
			wantTimeout: 2 * time.Minute,
		},
		{
			name: "explicit timeout",
			environment: map[string]string{
				"MIGRATION_DATABASE_URL": "postgres://database.example/the_search",
				"MIGRATION_TIMEOUT":      "45s",
			},
			wantTimeout: 45 * time.Second,
		},
		{name: "missing database URL", wantErr: "MIGRATION_DATABASE_URL is required"},
		{
			name: "blank database URL",
			environment: map[string]string{
				"MIGRATION_DATABASE_URL": "   ",
			},
			wantErr: "MIGRATION_DATABASE_URL is required",
		},
		{
			name: "invalid timeout",
			environment: map[string]string{
				"MIGRATION_DATABASE_URL": "postgres://user:" + credentialMarker + "@database.example/the_search",
				"MIGRATION_TIMEOUT":      "eventually",
			},
			wantErr: "MIGRATION_TIMEOUT",
		},
		{
			name: "non-positive timeout",
			environment: map[string]string{
				"MIGRATION_DATABASE_URL": "postgres://database.example/the_search",
				"MIGRATION_TIMEOUT":      "0s",
			},
			wantErr: "must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lookup := func(key string) (string, bool) {
				value, ok := tt.environment[key]
				return value, ok
			}
			config, err := migrations.ConfigFromEnvironment(lookup)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ConfigFromEnvironment() error = %v, want containing %q", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), credentialMarker) {
					t.Fatalf("ConfigFromEnvironment() error leaked database credentials: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConfigFromEnvironment() error = %v", err)
			}
			if config.DatabaseURL != tt.environment["MIGRATION_DATABASE_URL"] {
				t.Fatalf("DatabaseURL = %q, want configured value", config.DatabaseURL)
			}
			if config.Timeout != tt.wantTimeout {
				t.Fatalf("Timeout = %s, want %s", config.Timeout, tt.wantTimeout)
			}
		})
	}
}
