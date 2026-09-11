package forecast

import (
	"math"
	"strings"
	"testing"
)

func TestNewPointValidatesAndNormalizesReference(t *testing.T) {
	t.Parallel()

	got, err := NewPoint(" ericeira-offshore ", -9.417, 38.963)
	if err != nil {
		t.Fatalf("NewPoint() error = %v", err)
	}
	if got.Reference != "ericeira-offshore" || got.Longitude != -9.417 || got.Latitude != 38.963 {
		t.Fatalf("NewPoint() = %+v", got)
	}
}

func TestNewPointRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reference string
		longitude float64
		latitude  float64
	}{
		{name: "missing reference", longitude: -9.417, latitude: 38.963},
		{name: "long reference", reference: strings.Repeat("a", 201), longitude: -9.417, latitude: 38.963},
		{name: "nul reference", reference: "spot\x00one", longitude: -9.417, latitude: 38.963},
		{name: "longitude too low", reference: "spot", longitude: -180.1, latitude: 38.963},
		{name: "longitude too high", reference: "spot", longitude: 180.1, latitude: 38.963},
		{name: "longitude not finite", reference: "spot", longitude: math.NaN(), latitude: 38.963},
		{name: "latitude too low", reference: "spot", longitude: -9.417, latitude: -90.1},
		{name: "latitude too high", reference: "spot", longitude: -9.417, latitude: 90.1},
		{name: "latitude not finite", reference: "spot", longitude: -9.417, latitude: math.Inf(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewPoint(tt.reference, tt.longitude, tt.latitude); err == nil {
				t.Fatal("NewPoint() error = nil")
			}
		})
	}
}

func TestMeasurementDistinguishesZeroFromMissing(t *testing.T) {
	t.Parallel()

	zero, err := NewMeasurement(0)
	if err != nil {
		t.Fatalf("NewMeasurement() error = %v", err)
	}
	if !zero.Available || zero.Value != 0 {
		t.Fatalf("NewMeasurement(0) = %+v", zero)
	}
	if missing := MissingMeasurement(); missing.Available {
		t.Fatalf("MissingMeasurement() = %+v", missing)
	}
	if _, err := NewMeasurement(math.Inf(-1)); err == nil {
		t.Fatal("NewMeasurement(-Inf) error = nil")
	}
}
