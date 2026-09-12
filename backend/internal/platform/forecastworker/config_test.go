package forecastworker

import (
	"strings"
	"testing"
	"time"
)

func TestConfigFromEnvironmentUsesBoundedDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"FORECAST_DATABASE_URL":    "postgres://ingester@database/the_search",
		"FORECAST_HORIZON":         "48h",
		"FORECAST_TIMEOUT":         "25s",
		"OPEN_METEO_DAILY_LIMIT":   "4000",
		"OPEN_METEO_MONTHLY_LIMIT": "120000",
		"OPEN_METEO_ACCOUNT_SCOPE": "contract-a",
		"OPEN_METEO_FORECAST_URL":  "https://weather.example/v1/forecast",
	}
	config, err := ConfigFromEnvironment(mapLookup(values))
	if err != nil {
		t.Fatal(err)
	}
	if config.Horizon != 48*time.Hour || config.Timeout != 25*time.Second ||
		config.OpenMeteo.DailyLimit != 4000 || config.OpenMeteo.MonthlyLimit != 120000 ||
		config.OpenMeteo.AccountScope != "contract-a" || config.OpenMeteo.ForecastURL != values["OPEN_METEO_FORECAST_URL"] {
		t.Fatalf("ConfigFromEnvironment() = %+v", config)
	}
}

func TestConfigFromEnvironmentRejectsInvalidValuesWithoutLeakingDSN(t *testing.T) {
	t.Parallel()
	tests := []map[string]string{
		{},
		{"FORECAST_DATABASE_URL": "postgres://secret@database/db", "FORECAST_HORIZON": "90m"},
		{"FORECAST_DATABASE_URL": "postgres://secret@database/db", "FORECAST_HORIZON": "400h"},
		{"FORECAST_DATABASE_URL": "postgres://secret@database/db", "FORECAST_TIMEOUT": "never"},
		{"FORECAST_DATABASE_URL": "postgres://secret@database/db", "OPEN_METEO_DAILY_LIMIT": "0"},
	}
	for _, values := range tests {
		if _, err := ConfigFromEnvironment(mapLookup(values)); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("ConfigFromEnvironment(%v) error = %v", values, err)
		}
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}
}
