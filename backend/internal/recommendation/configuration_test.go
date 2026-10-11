package recommendation

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

var testOwner = identity.PrincipalID(uuid.MustParse("11111111-1111-4111-8111-111111111111"))

func mustRange(t *testing.T, minimum, maximum int) IntegerRange {
	t.Helper()
	valueRange, err := NewIntegerRange(minimum, maximum)
	if err != nil {
		t.Fatalf("new range: %v", err)
	}
	return valueRange
}

func mustSector(t *testing.T, start, end int, full bool) DirectionSector {
	t.Helper()
	sector, err := NewDirectionSector(start, end, full)
	if err != nil {
		t.Fatalf("new sector: %v", err)
	}
	return sector
}

func TestDirectionSectorContainsCircularBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sector    DirectionSector
		direction int
		want      bool
	}{
		{name: "ordinary start", sector: mustSector(t, 90_000, 180_000, false), direction: 90_000, want: true},
		{name: "ordinary end", sector: mustSector(t, 90_000, 180_000, false), direction: 180_000, want: true},
		{name: "ordinary outside", sector: mustSector(t, 90_000, 180_000, false), direction: 89_999},
		{name: "wrapped north", sector: mustSector(t, 330_000, 30_000, false), direction: 0, want: true},
		{name: "wrapped high", sector: mustSector(t, 330_000, 30_000, false), direction: 350_000, want: true},
		{name: "wrapped outside", sector: mustSector(t, 330_000, 30_000, false), direction: 180_000},
		{name: "single direction", sector: mustSector(t, 42_000, 42_000, false), direction: 42_000, want: true},
		{name: "single direction outside", sector: mustSector(t, 42_000, 42_000, false), direction: 42_001},
		{name: "full circle", sector: mustSector(t, 0, 0, true), direction: 215_000, want: true},
		{name: "invalid direction", sector: mustSector(t, 0, 0, true), direction: 360_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.sector.Contains(tt.direction); got != tt.want {
				t.Fatalf("contains(%d) = %v, want %v", tt.direction, got, tt.want)
			}
		})
	}
}

func TestDirectionSectorRejectsInvalidRepresentation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		start, end int
		full       bool
	}{
		{name: "negative", start: -1},
		{name: "upper bound", start: 0, end: 360_000},
		{name: "full circle with endpoints", start: 1, end: 1, full: true},
		{name: "implicit full circle", start: 0, end: 359_999},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewDirectionSector(tt.start, tt.end, tt.full); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDirectionSectorOverlapIncludesWrappedAndSharedBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		left, right DirectionSector
		want        bool
	}{
		{name: "separate", left: mustSector(t, 10_000, 20_000, false), right: mustSector(t, 21_000, 30_000, false)},
		{name: "shared inclusive boundary", left: mustSector(t, 10_000, 20_000, false), right: mustSector(t, 20_000, 30_000, false), want: true},
		{name: "wrapped overlap", left: mustSector(t, 330_000, 30_000, false), right: mustSector(t, 20_000, 40_000, false), want: true},
		{name: "full circle", left: mustSector(t, 0, 0, true), right: mustSector(t, 120_000, 180_000, false), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.left.Overlaps(tt.right); got != tt.want {
				t.Fatalf("overlaps = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewSpotInterpretationValidatesContradictorySectors(t *testing.T) {
	t.Parallel()

	waveRange := mustRange(t, 300, 3_000)
	periodRange := mustRange(t, 6_000, 20_000)
	validWind := []WindResponse{{Sector: mustSector(t, 0, 0, true), Relationship: WindCrossShore}}

	tests := []struct {
		name   string
		swell  []SwellExposure
		wind   []WindResponse
		limits []string
	}{
		{
			name: "overlapping swell",
			swell: []SwellExposure{
				{Sector: mustSector(t, 330_000, 30_000, false), ExposureBasisPoints: 8_000},
				{Sector: mustSector(t, 20_000, 40_000, false), ExposureBasisPoints: 5_000},
			},
			wind: validWind,
		},
		{
			name:   "invalid limitation whitespace",
			swell:  []SwellExposure{{Sector: mustSector(t, 0, 0, true), ExposureBasisPoints: 8_000}},
			wind:   validWind,
			limits: []string{"  shallow reef"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewSpotInterpretation(testOwner, uuid.New(), tt.swell, tt.wind, waveRange, periodRange, InterpretationConfidenceMedium, tt.limits); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNewSpotInterpretationDefensivelyCopiesSlices(t *testing.T) {
	t.Parallel()

	swell := []SwellExposure{{Sector: mustSector(t, 0, 0, true), ExposureBasisPoints: 8_000}}
	wind := []WindResponse{{Sector: mustSector(t, 0, 0, true), Relationship: WindOffshore}}
	limitations := []string{"Uncalibrated offshore grid input"}
	interpretation, err := NewSpotInterpretation(testOwner, uuid.New(), swell, wind, mustRange(t, 300, 3_000), mustRange(t, 6_000, 20_000), InterpretationConfidenceLow, limitations)
	if err != nil {
		t.Fatalf("new interpretation: %v", err)
	}

	swell[0].ExposureBasisPoints = 1
	wind[0].Relationship = WindOnshore
	limitations[0] = "changed"
	if interpretation.SwellExposures[0].ExposureBasisPoints != 8_000 || interpretation.WindResponses[0].Relationship != WindOffshore || interpretation.Limitations[0] != "Uncalibrated offshore grid input" {
		t.Fatal("constructor retained caller-owned slice storage")
	}
}

func TestPreferencesRejectPreferredValuesOutsideComfort(t *testing.T) {
	t.Parallel()

	comfortHeight := mustRange(t, 500, 2_500)
	preferredHeight := mustRange(t, 800, 1_800)
	comfortPeriod := mustRange(t, 6_000, 18_000)
	preferredPeriod := mustRange(t, 9_000, 14_000)

	tests := []struct {
		name                 string
		scope                PreferenceScope
		spotID               uuid.UUID
		comfortHeight        IntegerRange
		preferredHeight      IntegerRange
		comfortPeriod        IntegerRange
		preferredPeriod      IntegerRange
		maximumWind          int
		preferredMaximumWind int
	}{
		{name: "valid global", scope: PreferencesGlobal, comfortHeight: comfortHeight, preferredHeight: preferredHeight, comfortPeriod: comfortPeriod, preferredPeriod: preferredPeriod, maximumWind: 12_000, preferredMaximumWind: 7_000},
		{name: "valid spot", scope: PreferencesSpot, spotID: uuid.New(), comfortHeight: comfortHeight, preferredHeight: preferredHeight, comfortPeriod: comfortPeriod, preferredPeriod: preferredPeriod, maximumWind: 12_000, preferredMaximumWind: 7_000},
		{name: "global identifies spot", scope: PreferencesGlobal, spotID: uuid.New(), comfortHeight: comfortHeight, preferredHeight: preferredHeight, comfortPeriod: comfortPeriod, preferredPeriod: preferredPeriod, maximumWind: 12_000, preferredMaximumWind: 7_000},
		{name: "spot missing ID", scope: PreferencesSpot, comfortHeight: comfortHeight, preferredHeight: preferredHeight, comfortPeriod: comfortPeriod, preferredPeriod: preferredPeriod, maximumWind: 12_000, preferredMaximumWind: 7_000},
		{name: "height outside", scope: PreferencesGlobal, comfortHeight: comfortHeight, preferredHeight: mustRange(t, 400, 1_800), comfortPeriod: comfortPeriod, preferredPeriod: preferredPeriod, maximumWind: 12_000, preferredMaximumWind: 7_000},
		{name: "period outside", scope: PreferencesGlobal, comfortHeight: comfortHeight, preferredHeight: preferredHeight, comfortPeriod: comfortPeriod, preferredPeriod: mustRange(t, 9_000, 19_000), maximumWind: 12_000, preferredMaximumWind: 7_000},
		{name: "wind outside", scope: PreferencesGlobal, comfortHeight: comfortHeight, preferredHeight: preferredHeight, comfortPeriod: comfortPeriod, preferredPeriod: preferredPeriod, maximumWind: 12_000, preferredMaximumWind: 13_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewPreferences(testOwner, tt.scope, tt.spotID, tt.comfortHeight, tt.preferredHeight, tt.comfortPeriod, tt.preferredPeriod, tt.maximumWind, tt.preferredMaximumWind)
			wantError := tt.name != "valid global" && tt.name != "valid spot"
			if (err != nil) != wantError {
				t.Fatalf("error = %v, wantError %v", err, wantError)
			}
		})
	}
}

func TestSpotForecastMappingValidatesWeightsRolesAndLifecycle(t *testing.T) {
	t.Parallel()

	primaryID := uuid.New()
	secondaryID := uuid.New()
	validFrom := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	valid := []ForecastPointMapping{
		{ForecastPointID: primaryID, Role: MappingPrimary, WeightBasisPoints: 7_000, SelectionProvenance: "nearest-sea-grid-v1"},
		{ForecastPointID: secondaryID, Role: MappingSecondary, WeightBasisPoints: 3_000, SelectionProvenance: "shelter-reference-v1"},
	}

	tests := []struct {
		name       string
		points     []ForecastPointMapping
		validUntil time.Time
		wantError  bool
	}{
		{name: "valid open ended", points: valid},
		{name: "valid closed interval", points: valid, validUntil: validFrom.Add(time.Hour)},
		{name: "wrong total", points: []ForecastPointMapping{{ForecastPointID: primaryID, Role: MappingPrimary, WeightBasisPoints: 9_999, SelectionProvenance: "nearest-sea-grid-v1"}}, wantError: true},
		{name: "duplicate point", points: []ForecastPointMapping{{ForecastPointID: primaryID, Role: MappingPrimary, WeightBasisPoints: 5_000, SelectionProvenance: "nearest-sea-grid-v1"}, {ForecastPointID: primaryID, Role: MappingSecondary, WeightBasisPoints: 5_000, SelectionProvenance: "nearest-sea-grid-v1"}}, wantError: true},
		{name: "two primary points", points: []ForecastPointMapping{{ForecastPointID: primaryID, Role: MappingPrimary, WeightBasisPoints: 5_000, SelectionProvenance: "nearest-sea-grid-v1"}, {ForecastPointID: secondaryID, Role: MappingPrimary, WeightBasisPoints: 5_000, SelectionProvenance: "nearest-sea-grid-v1"}}, wantError: true},
		{name: "invalid lifecycle", points: valid, validUntil: validFrom, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mapping, err := NewSpotForecastMapping(testOwner, uuid.New(), tt.points, validFrom, tt.validUntil)
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, wantError %v", err, tt.wantError)
			}
			if err == nil && (!mapping.ActiveAt(validFrom) || mapping.ActiveAt(validFrom.Add(-time.Nanosecond))) {
				t.Fatal("mapping validity interval is not start-inclusive")
			}
		})
	}
}

func TestAssessReadinessReturnsStableCompleteReasons(t *testing.T) {
	t.Parallel()

	got := AssessReadiness(ReadinessInput{})
	want := []ReadinessReason{
		ReadinessMissingMapping,
		ReadinessMissingSpotInterpretation,
		ReadinessMissingPreferences,
		ReadinessMissingSelectedBoard,
	}
	if got.Ready || !reflect.DeepEqual(got.Reasons, want) {
		t.Fatalf("readiness = %#v, want reasons %#v", got, want)
	}

	ready := AssessReadiness(ReadinessInput{HasMapping: true, HasSpotInterpretation: true, HasPreferences: true, SelectedBoardID: uuid.New()})
	if !ready.Ready || len(ready.Reasons) != 0 {
		t.Fatalf("ready result = %#v", ready)
	}
}

func TestRestoreAggregatesRejectInvalidHistory(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	boardID := uuid.New()
	height := mustRange(t, 500, 2_500)
	period := mustRange(t, 6_000, 18_000)

	for _, tt := range []struct {
		name      string
		version   int64
		createdAt time.Time
		updatedAt time.Time
	}{
		{name: "nonpositive version", createdAt: createdAt, updatedAt: createdAt},
		{name: "missing timestamp", version: 1},
		{name: "reversed timestamps", version: 1, createdAt: createdAt, updatedAt: createdAt.Add(-time.Second)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := RestoreBoard(boardID, testOwner, "Daily driver", height, period, tt.version, tt.createdAt, tt.updatedAt); err == nil {
				t.Fatal("expected restore validation error")
			}
		})
	}
}

func TestFiniteDecimalToIntegerUsesContractRoundingAndRejectsNonFinite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     float64
		scale     float64
		want      int
		wantError bool
	}{
		{name: "positive half away", value: 1.2345, scale: 1_000, want: 1_235},
		{name: "negative half away", value: -1.2345, scale: 1_000, want: -1_235},
		{name: "nan", value: math.NaN(), scale: 1_000, wantError: true},
		{name: "infinite scale", value: 1, scale: math.Inf(1), wantError: true},
		{name: "nonpositive scale", value: 1, scale: 0, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := FiniteDecimalToInteger(tt.value, tt.scale)
			if (err != nil) != tt.wantError || got != tt.want {
				t.Fatalf("result = %d, error = %v; want %d, wantError %v", got, err, tt.want, tt.wantError)
			}
		})
	}
}
