package forecasting

import (
	"errors"
	"testing"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

func TestNewFetchRequestNormalizesUTCAndCopiesPoints(t *testing.T) {
	t.Parallel()

	point := mustPoint(t, "ericeira")
	points := []forecast.Point{point}
	location := time.FixedZone("WEST", 60*60)
	start := time.Date(2026, time.September, 11, 9, 0, 0, 0, location)

	got, err := NewFetchRequest(points, start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("NewFetchRequest() error = %v", err)
	}
	points[0].Reference = "mutated"
	if got.Points[0].Reference != point.Reference {
		t.Fatal("NewFetchRequest() retained caller-owned point storage")
	}
	if got.StartsAt.Location() != time.UTC || got.EndsAt.Location() != time.UTC {
		t.Fatalf("NewFetchRequest() window = (%v, %v), want UTC", got.StartsAt, got.EndsAt)
	}
	if got.StartsAt.Hour() != 8 {
		t.Fatalf("NewFetchRequest() start hour = %d, want 8 UTC", got.StartsAt.Hour())
	}
}

func TestNewFetchRequestRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	point := mustPoint(t, "ericeira")
	start := time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		points []forecast.Point
		start  time.Time
		end    time.Time
	}{
		{name: "no points", start: start, end: start.Add(time.Hour)},
		{name: "uninitialized point", points: []forecast.Point{{}}, start: start, end: start.Add(time.Hour)},
		{name: "invalid point coordinates", points: []forecast.Point{{Reference: "spot", Longitude: -181, Latitude: 38.963}}, start: start, end: start.Add(time.Hour)},
		{name: "duplicate point reference", points: []forecast.Point{point, point}, start: start, end: start.Add(time.Hour)},
		{name: "zero start", points: []forecast.Point{point}, end: start.Add(time.Hour)},
		{name: "empty window", points: []forecast.Point{point}, start: start, end: start},
		{name: "reversed window", points: []forecast.Point{point}, start: start, end: start.Add(-time.Hour)},
		{name: "unaligned start", points: []forecast.Point{point}, start: start.Add(time.Minute), end: start.Add(time.Hour)},
		{name: "unaligned end", points: []forecast.Point{point}, start: start, end: start.Add(time.Hour + time.Second)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewFetchRequest(tt.points, tt.start, tt.end); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("NewFetchRequest() error = %v, want invalid request", err)
			}
		})
	}
}

func mustPoint(t *testing.T, reference string) forecast.Point {
	t.Helper()
	point, err := forecast.NewPoint(reference, -9.417, 38.963)
	if err != nil {
		t.Fatal(err)
	}
	return point
}
