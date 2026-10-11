// Package recommendation defines recommendation-domain configuration and invariants.
package recommendation

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

const (
	fullWeightBasisPoints = 10_000
	maxDirectionMilliDeg  = 359_999
	maxBoardNameLength    = 120
	maxProvenanceLength   = 200
	maxLimitationLength   = 300
	maxLimitations        = 20
)

// IntegerRange is an inclusive range in the unit named by its containing field.
type IntegerRange struct {
	Minimum int
	Maximum int
}

// NewIntegerRange validates a non-negative inclusive canonical-unit range.
func NewIntegerRange(minimum, maximum int) (IntegerRange, error) {
	if minimum < 0 || maximum < minimum {
		return IntegerRange{}, errors.New("range must be non-negative and ordered")
	}
	return IntegerRange{Minimum: minimum, Maximum: maximum}, nil
}

// Contains reports whether value is inside the inclusive range.
func (valueRange IntegerRange) Contains(value int) bool {
	return value >= valueRange.Minimum && value <= valueRange.Maximum
}

// ContainsRange reports whether another range is fully inside this range.
func (valueRange IntegerRange) ContainsRange(other IntegerRange) bool {
	return valueRange.Contains(other.Minimum) && valueRange.Contains(other.Maximum)
}

// DirectionSector is an inclusive circular sector in integer milli-degrees.
// Directions are clockwise from true north and describe where wind or waves
// arrive from. A full circle is explicit; equal non-full endpoints describe one
// exact direction.
type DirectionSector struct {
	StartMilliDegrees int
	EndMilliDegrees   int
	FullCircle        bool
}

// NewDirectionSector validates an inclusive circular sector.
func NewDirectionSector(startMilliDegrees, endMilliDegrees int, fullCircle bool) (DirectionSector, error) {
	sector := DirectionSector{
		StartMilliDegrees: startMilliDegrees,
		EndMilliDegrees:   endMilliDegrees,
		FullCircle:        fullCircle,
	}
	if err := sector.Validate(); err != nil {
		return DirectionSector{}, err
	}
	return sector, nil
}

// Validate checks the canonical representation of a direction sector.
func (sector DirectionSector) Validate() error {
	if sector.StartMilliDegrees < 0 || sector.StartMilliDegrees > maxDirectionMilliDeg ||
		sector.EndMilliDegrees < 0 || sector.EndMilliDegrees > maxDirectionMilliDeg {
		return errors.New("direction endpoints must be between 0 and 359999 milli-degrees")
	}
	if sector.FullCircle && (sector.StartMilliDegrees != 0 || sector.EndMilliDegrees != 0) {
		return errors.New("a full-circle direction sector must use zero endpoints")
	}
	if !sector.FullCircle && sector.StartMilliDegrees == 0 && sector.EndMilliDegrees == maxDirectionMilliDeg {
		return errors.New("a full-circle direction sector must be explicit")
	}
	return nil
}

// Contains reports whether a canonical direction is inside the inclusive sector.
func (sector DirectionSector) Contains(directionMilliDegrees int) bool {
	if directionMilliDegrees < 0 || directionMilliDegrees > maxDirectionMilliDeg {
		return false
	}
	if sector.FullCircle {
		return true
	}
	if sector.StartMilliDegrees <= sector.EndMilliDegrees {
		return directionMilliDegrees >= sector.StartMilliDegrees && directionMilliDegrees <= sector.EndMilliDegrees
	}
	return directionMilliDegrees >= sector.StartMilliDegrees || directionMilliDegrees <= sector.EndMilliDegrees
}

// Overlaps reports whether two inclusive sectors contain a common direction.
func (sector DirectionSector) Overlaps(other DirectionSector) bool {
	for _, left := range sector.linearSegments() {
		for _, right := range other.linearSegments() {
			if left[0] <= right[1] && right[0] <= left[1] {
				return true
			}
		}
	}
	return false
}

func (sector DirectionSector) linearSegments() [][2]int {
	if sector.FullCircle {
		return [][2]int{{0, maxDirectionMilliDeg}}
	}
	if sector.StartMilliDegrees <= sector.EndMilliDegrees {
		return [][2]int{{sector.StartMilliDegrees, sector.EndMilliDegrees}}
	}
	return [][2]int{{sector.StartMilliDegrees, maxDirectionMilliDeg}, {0, sector.EndMilliDegrees}}
}

// InterpretationConfidence expresses the maturity of user-provided local knowledge.
type InterpretationConfidence string

// Supported spot-interpretation confidence levels.
const (
	InterpretationConfidenceLow    InterpretationConfidence = "low"
	InterpretationConfidenceMedium InterpretationConfidence = "medium"
	InterpretationConfidenceHigh   InterpretationConfidence = "high"
)

// WindRelationship describes a local spot's response to wind arriving from a sector.
type WindRelationship string

// Supported wind relationships. These classify local knowledge; they do not
// themselves assert a score or safety outcome.
const (
	WindOffshore   WindRelationship = "offshore"
	WindCrossShore WindRelationship = "cross_shore"
	WindOnshore    WindRelationship = "onshore"
	WindSheltered  WindRelationship = "sheltered"
)

// SwellExposure describes how strongly a spot receives swell from a sector.
// ExposureBasisPoints is an interpretation index, not an at-break height multiplier.
type SwellExposure struct {
	Sector              DirectionSector
	ExposureBasisPoints int
}

// WindResponse associates a direction sector with a local wind relationship.
type WindResponse struct {
	Sector       DirectionSector
	Relationship WindRelationship
}

// SpotInterpretation is immutable local knowledge for one spot version.
type SpotInterpretation struct {
	OwnerID                          identity.PrincipalID
	SpotID                           uuid.UUID
	SwellExposures                   []SwellExposure
	WindResponses                    []WindResponse
	SupportedWaveHeightMillimetres   IntegerRange
	SupportedSwellPeriodMilliseconds IntegerRange
	Confidence                       InterpretationConfidence
	Limitations                      []string
	Version                          int64
	CreatedAt                        time.Time
	UpdatedAt                        time.Time
}

// NewSpotInterpretation validates local spot knowledge. Direction coverage may
// contain gaps, which remain unknown, but contradictory overlapping sectors are rejected.
func NewSpotInterpretation(
	owner identity.PrincipalID,
	spotID uuid.UUID,
	swellExposures []SwellExposure,
	windResponses []WindResponse,
	waveHeightMillimetres IntegerRange,
	swellPeriodMilliseconds IntegerRange,
	confidence InterpretationConfidence,
	limitations []string,
) (SpotInterpretation, error) {
	interpretation := SpotInterpretation{
		OwnerID:                          owner,
		SpotID:                           spotID,
		SwellExposures:                   append([]SwellExposure(nil), swellExposures...),
		WindResponses:                    append([]WindResponse(nil), windResponses...),
		SupportedWaveHeightMillimetres:   waveHeightMillimetres,
		SupportedSwellPeriodMilliseconds: swellPeriodMilliseconds,
		Confidence:                       confidence,
		Limitations:                      append([]string(nil), limitations...),
		Version:                          1,
	}
	if err := interpretation.validate(); err != nil {
		return SpotInterpretation{}, err
	}
	return interpretation, nil
}

// RestoreSpotInterpretation validates persisted spot-interpretation state.
func RestoreSpotInterpretation(
	owner identity.PrincipalID,
	spotID uuid.UUID,
	swellExposures []SwellExposure,
	windResponses []WindResponse,
	waveHeightMillimetres IntegerRange,
	swellPeriodMilliseconds IntegerRange,
	confidence InterpretationConfidence,
	limitations []string,
	version int64,
	createdAt, updatedAt time.Time,
) (SpotInterpretation, error) {
	interpretation, err := NewSpotInterpretation(owner, spotID, swellExposures, windResponses, waveHeightMillimetres, swellPeriodMilliseconds, confidence, limitations)
	if err != nil {
		return SpotInterpretation{}, err
	}
	if err := validateRestoredAggregate(version, createdAt, updatedAt); err != nil {
		return SpotInterpretation{}, err
	}
	interpretation.Version = version
	interpretation.CreatedAt = createdAt
	interpretation.UpdatedAt = updatedAt
	return interpretation, nil
}

func (interpretation SpotInterpretation) validate() error {
	if interpretation.OwnerID.IsZero() {
		return errors.New("spot interpretation owner is required")
	}
	if interpretation.SpotID == uuid.Nil {
		return errors.New("spot interpretation spot is required")
	}
	if len(interpretation.SwellExposures) == 0 {
		return errors.New("at least one swell exposure is required")
	}
	for index, exposure := range interpretation.SwellExposures {
		if err := exposure.Sector.Validate(); err != nil {
			return errors.New("invalid swell exposure sector")
		}
		if exposure.ExposureBasisPoints < 0 || exposure.ExposureBasisPoints > fullWeightBasisPoints {
			return errors.New("swell exposure must be between 0 and 10000 basis points")
		}
		for previous := range index {
			if exposure.Sector.Overlaps(interpretation.SwellExposures[previous].Sector) {
				return errors.New("swell exposure sectors must not overlap")
			}
		}
	}
	if len(interpretation.WindResponses) == 0 {
		return errors.New("at least one wind response is required")
	}
	for index, response := range interpretation.WindResponses {
		if err := response.Sector.Validate(); err != nil {
			return errors.New("invalid wind response sector")
		}
		if !validWindRelationship(response.Relationship) {
			return errors.New("unsupported wind relationship")
		}
		for previous := range index {
			if response.Sector.Overlaps(interpretation.WindResponses[previous].Sector) {
				return errors.New("wind response sectors must not overlap")
			}
		}
	}
	if _, err := NewIntegerRange(interpretation.SupportedWaveHeightMillimetres.Minimum, interpretation.SupportedWaveHeightMillimetres.Maximum); err != nil {
		return errors.New("invalid supported wave-height range")
	}
	if _, err := NewIntegerRange(interpretation.SupportedSwellPeriodMilliseconds.Minimum, interpretation.SupportedSwellPeriodMilliseconds.Maximum); err != nil {
		return errors.New("invalid supported swell-period range")
	}
	if interpretation.SupportedSwellPeriodMilliseconds.Minimum == 0 {
		return errors.New("supported swell period must be positive")
	}
	if !validInterpretationConfidence(interpretation.Confidence) {
		return errors.New("unsupported interpretation confidence")
	}
	if len(interpretation.Limitations) > maxLimitations {
		return errors.New("spot interpretation has too many limitations")
	}
	for _, limitation := range interpretation.Limitations {
		if err := validateNormalizedText(limitation, maxLimitationLength); err != nil {
			return errors.New("invalid spot interpretation limitation")
		}
	}
	return nil
}

func validWindRelationship(value WindRelationship) bool {
	switch value {
	case WindOffshore, WindCrossShore, WindOnshore, WindSheltered:
		return true
	default:
		return false
	}
}

func validInterpretationConfidence(value InterpretationConfidence) bool {
	switch value {
	case InterpretationConfidenceLow, InterpretationConfidenceMedium, InterpretationConfidenceHigh:
		return true
	default:
		return false
	}
}

// Board is an owner-scoped equipment capability envelope. It deliberately
// avoids inferring capability from category, dimensions, or volume.
type Board struct {
	ID                            uuid.UUID
	OwnerID                       identity.PrincipalID
	Name                          string
	UsableWaveHeightMillimetres   IntegerRange
	UsableSwellPeriodMilliseconds IntegerRange
	Version                       int64
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

// NewBoard validates and normalizes a user-declared board envelope.
func NewBoard(id uuid.UUID, owner identity.PrincipalID, name string, waveHeightMillimetres, swellPeriodMilliseconds IntegerRange) (Board, error) {
	board := Board{
		ID:                            id,
		OwnerID:                       owner,
		Name:                          strings.TrimSpace(name),
		UsableWaveHeightMillimetres:   waveHeightMillimetres,
		UsableSwellPeriodMilliseconds: swellPeriodMilliseconds,
		Version:                       1,
	}
	if err := board.validate(); err != nil {
		return Board{}, err
	}
	return board, nil
}

// RestoreBoard validates persisted board state.
func RestoreBoard(id uuid.UUID, owner identity.PrincipalID, name string, waveHeightMillimetres, swellPeriodMilliseconds IntegerRange, version int64, createdAt, updatedAt time.Time) (Board, error) {
	board, err := NewBoard(id, owner, name, waveHeightMillimetres, swellPeriodMilliseconds)
	if err != nil {
		return Board{}, err
	}
	if err := validateRestoredAggregate(version, createdAt, updatedAt); err != nil {
		return Board{}, err
	}
	board.Version = version
	board.CreatedAt = createdAt
	board.UpdatedAt = updatedAt
	return board, nil
}

func (board Board) validate() error {
	if board.ID == uuid.Nil {
		return errors.New("board ID is required")
	}
	if board.OwnerID.IsZero() {
		return errors.New("board owner is required")
	}
	if err := validateNormalizedText(board.Name, maxBoardNameLength); err != nil {
		return errors.New("board name must contain between 1 and 120 normalized characters")
	}
	if _, err := NewIntegerRange(board.UsableWaveHeightMillimetres.Minimum, board.UsableWaveHeightMillimetres.Maximum); err != nil {
		return errors.New("invalid board wave-height range")
	}
	if _, err := NewIntegerRange(board.UsableSwellPeriodMilliseconds.Minimum, board.UsableSwellPeriodMilliseconds.Maximum); err != nil {
		return errors.New("invalid board swell-period range")
	}
	if board.UsableSwellPeriodMilliseconds.Minimum == 0 {
		return errors.New("board swell period must be positive")
	}
	return nil
}

// PreferenceScope distinguishes global defaults from explicit per-spot overrides.
type PreferenceScope string

// Supported preference scopes.
const (
	PreferencesGlobal PreferenceScope = "global"
	PreferencesSpot   PreferenceScope = "spot"
)

// Preferences stores explicit comfort limits and softer preferred ranges.
type Preferences struct {
	OwnerID                                  identity.PrincipalID
	Scope                                    PreferenceScope
	SpotID                                   uuid.UUID
	ComfortWaveHeightMillimetres             IntegerRange
	PreferredWaveHeightMillimetres           IntegerRange
	ComfortSwellPeriodMilliseconds           IntegerRange
	PreferredSwellPeriodMilliseconds         IntegerRange
	MaximumComfortWindMillimetresPerSecond   int
	PreferredMaximumWindMillimetresPerSecond int
	Version                                  int64
	CreatedAt                                time.Time
	UpdatedAt                                time.Time
}

// NewPreferences validates global defaults or an explicit per-spot override.
// SpotID must be nil for global preferences and non-nil for spot preferences.
func NewPreferences(
	owner identity.PrincipalID,
	scope PreferenceScope,
	spotID uuid.UUID,
	comfortWaveHeightMillimetres, preferredWaveHeightMillimetres IntegerRange,
	comfortSwellPeriodMilliseconds, preferredSwellPeriodMilliseconds IntegerRange,
	maximumComfortWindMillimetresPerSecond, preferredMaximumWindMillimetresPerSecond int,
) (Preferences, error) {
	preferences := Preferences{
		OwnerID:                                  owner,
		Scope:                                    scope,
		SpotID:                                   spotID,
		ComfortWaveHeightMillimetres:             comfortWaveHeightMillimetres,
		PreferredWaveHeightMillimetres:           preferredWaveHeightMillimetres,
		ComfortSwellPeriodMilliseconds:           comfortSwellPeriodMilliseconds,
		PreferredSwellPeriodMilliseconds:         preferredSwellPeriodMilliseconds,
		MaximumComfortWindMillimetresPerSecond:   maximumComfortWindMillimetresPerSecond,
		PreferredMaximumWindMillimetresPerSecond: preferredMaximumWindMillimetresPerSecond,
		Version:                                  1,
	}
	if err := preferences.validate(); err != nil {
		return Preferences{}, err
	}
	return preferences, nil
}

// RestorePreferences validates persisted preference state.
func RestorePreferences(
	owner identity.PrincipalID,
	scope PreferenceScope,
	spotID uuid.UUID,
	comfortWaveHeightMillimetres, preferredWaveHeightMillimetres IntegerRange,
	comfortSwellPeriodMilliseconds, preferredSwellPeriodMilliseconds IntegerRange,
	maximumComfortWindMillimetresPerSecond, preferredMaximumWindMillimetresPerSecond int,
	version int64,
	createdAt, updatedAt time.Time,
) (Preferences, error) {
	preferences, err := NewPreferences(owner, scope, spotID, comfortWaveHeightMillimetres, preferredWaveHeightMillimetres, comfortSwellPeriodMilliseconds, preferredSwellPeriodMilliseconds, maximumComfortWindMillimetresPerSecond, preferredMaximumWindMillimetresPerSecond)
	if err != nil {
		return Preferences{}, err
	}
	if err := validateRestoredAggregate(version, createdAt, updatedAt); err != nil {
		return Preferences{}, err
	}
	preferences.Version = version
	preferences.CreatedAt = createdAt
	preferences.UpdatedAt = updatedAt
	return preferences, nil
}

func (preferences Preferences) validate() error {
	if preferences.OwnerID.IsZero() {
		return errors.New("preferences owner is required")
	}
	switch preferences.Scope {
	case PreferencesGlobal:
		if preferences.SpotID != uuid.Nil {
			return errors.New("global preferences must not identify a spot")
		}
	case PreferencesSpot:
		if preferences.SpotID == uuid.Nil {
			return errors.New("spot preferences require a spot")
		}
	default:
		return errors.New("unsupported preference scope")
	}
	if _, err := NewIntegerRange(preferences.ComfortWaveHeightMillimetres.Minimum, preferences.ComfortWaveHeightMillimetres.Maximum); err != nil {
		return errors.New("invalid comfort wave-height range")
	}
	if _, err := NewIntegerRange(preferences.PreferredWaveHeightMillimetres.Minimum, preferences.PreferredWaveHeightMillimetres.Maximum); err != nil {
		return errors.New("invalid preferred wave-height range")
	}
	if !preferences.ComfortWaveHeightMillimetres.ContainsRange(preferences.PreferredWaveHeightMillimetres) {
		return errors.New("preferred wave-height range must be inside comfort range")
	}
	if _, err := NewIntegerRange(preferences.ComfortSwellPeriodMilliseconds.Minimum, preferences.ComfortSwellPeriodMilliseconds.Maximum); err != nil {
		return errors.New("invalid comfort swell-period range")
	}
	if preferences.ComfortSwellPeriodMilliseconds.Minimum == 0 {
		return errors.New("comfort swell period must be positive")
	}
	if _, err := NewIntegerRange(preferences.PreferredSwellPeriodMilliseconds.Minimum, preferences.PreferredSwellPeriodMilliseconds.Maximum); err != nil {
		return errors.New("invalid preferred swell-period range")
	}
	if !preferences.ComfortSwellPeriodMilliseconds.ContainsRange(preferences.PreferredSwellPeriodMilliseconds) {
		return errors.New("preferred swell-period range must be inside comfort range")
	}
	if preferences.MaximumComfortWindMillimetresPerSecond < 0 ||
		preferences.PreferredMaximumWindMillimetresPerSecond < 0 ||
		preferences.PreferredMaximumWindMillimetresPerSecond > preferences.MaximumComfortWindMillimetresPerSecond {
		return errors.New("preferred maximum wind must be inside the comfort limit")
	}
	return nil
}

// MappingRole identifies how a forecast point contributes to a spot mapping.
type MappingRole string

// Supported forecast-point mapping roles.
const (
	MappingPrimary   MappingRole = "primary"
	MappingSecondary MappingRole = "secondary"
)

// ForecastPointMapping is one weighted input of a versioned spot mapping.
type ForecastPointMapping struct {
	ForecastPointID     uuid.UUID
	Role                MappingRole
	WeightBasisPoints   int
	SelectionProvenance string
}

// SpotForecastMapping is one immutable mapping version with an explicit validity interval.
type SpotForecastMapping struct {
	OwnerID    identity.PrincipalID
	SpotID     uuid.UUID
	Points     []ForecastPointMapping
	ValidFrom  time.Time
	ValidUntil time.Time
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewSpotForecastMapping validates a mapping version. A zero ValidUntil means
// the mapping is open-ended.
func NewSpotForecastMapping(owner identity.PrincipalID, spotID uuid.UUID, points []ForecastPointMapping, validFrom, validUntil time.Time) (SpotForecastMapping, error) {
	mapping := SpotForecastMapping{
		OwnerID:    owner,
		SpotID:     spotID,
		Points:     append([]ForecastPointMapping(nil), points...),
		ValidFrom:  validFrom,
		ValidUntil: validUntil,
		Version:    1,
	}
	if err := mapping.validate(); err != nil {
		return SpotForecastMapping{}, err
	}
	return mapping, nil
}

// RestoreSpotForecastMapping validates persisted mapping state.
func RestoreSpotForecastMapping(owner identity.PrincipalID, spotID uuid.UUID, points []ForecastPointMapping, validFrom, validUntil time.Time, version int64, createdAt, updatedAt time.Time) (SpotForecastMapping, error) {
	mapping, err := NewSpotForecastMapping(owner, spotID, points, validFrom, validUntil)
	if err != nil {
		return SpotForecastMapping{}, err
	}
	if err := validateRestoredAggregate(version, createdAt, updatedAt); err != nil {
		return SpotForecastMapping{}, err
	}
	mapping.Version = version
	mapping.CreatedAt = createdAt
	mapping.UpdatedAt = updatedAt
	return mapping, nil
}

func (mapping SpotForecastMapping) validate() error {
	if mapping.OwnerID.IsZero() {
		return errors.New("spot forecast mapping owner is required")
	}
	if mapping.SpotID == uuid.Nil {
		return errors.New("spot forecast mapping spot is required")
	}
	if mapping.ValidFrom.IsZero() {
		return errors.New("spot forecast mapping valid-from instant is required")
	}
	if !mapping.ValidUntil.IsZero() && !mapping.ValidUntil.After(mapping.ValidFrom) {
		return errors.New("spot forecast mapping valid-until must follow valid-from")
	}
	if len(mapping.Points) == 0 {
		return errors.New("spot forecast mapping requires at least one forecast point")
	}
	totalWeight := 0
	primaryCount := 0
	seen := make(map[uuid.UUID]struct{}, len(mapping.Points))
	for _, point := range mapping.Points {
		if point.ForecastPointID == uuid.Nil {
			return errors.New("mapped forecast point ID is required")
		}
		if _, exists := seen[point.ForecastPointID]; exists {
			return errors.New("mapped forecast point IDs must be unique")
		}
		seen[point.ForecastPointID] = struct{}{}
		switch point.Role {
		case MappingPrimary:
			primaryCount++
		case MappingSecondary:
		default:
			return errors.New("unsupported forecast point mapping role")
		}
		if point.WeightBasisPoints <= 0 || point.WeightBasisPoints > fullWeightBasisPoints {
			return errors.New("forecast point weight must be between 1 and 10000 basis points")
		}
		totalWeight += point.WeightBasisPoints
		if err := validateNormalizedText(point.SelectionProvenance, maxProvenanceLength); err != nil {
			return errors.New("invalid forecast point selection provenance")
		}
	}
	if primaryCount != 1 {
		return errors.New("spot forecast mapping requires exactly one primary point")
	}
	if totalWeight != fullWeightBasisPoints {
		return errors.New("forecast point weights must sum to 10000 basis points")
	}
	return nil
}

// ActiveAt reports whether the mapping version is valid for an instant.
func (mapping SpotForecastMapping) ActiveAt(instant time.Time) bool {
	return !instant.Before(mapping.ValidFrom) && (mapping.ValidUntil.IsZero() || instant.Before(mapping.ValidUntil))
}

// ReadinessReason is a stable machine-readable reason a favorite cannot yet be evaluated.
type ReadinessReason string

// Recommendation readiness reasons in deterministic presentation order.
const (
	ReadinessMissingMapping            ReadinessReason = "missing_mapping"
	ReadinessMissingSpotInterpretation ReadinessReason = "missing_spot_interpretation"
	ReadinessMissingPreferences        ReadinessReason = "missing_preferences"
	ReadinessMissingSelectedBoard      ReadinessReason = "missing_selected_board"
)

// ReadinessInput records configuration availability without loading mutable details.
type ReadinessInput struct {
	HasMapping            bool
	HasSpotInterpretation bool
	HasPreferences        bool
	SelectedBoardID       uuid.UUID
}

// Readiness is a deterministic recommendation-configuration readiness result.
type Readiness struct {
	Ready   bool
	Reasons []ReadinessReason
}

// AssessReadiness reports every missing prerequisite in stable contract order.
func AssessReadiness(input ReadinessInput) Readiness {
	reasons := make([]ReadinessReason, 0, 4)
	if !input.HasMapping {
		reasons = append(reasons, ReadinessMissingMapping)
	}
	if !input.HasSpotInterpretation {
		reasons = append(reasons, ReadinessMissingSpotInterpretation)
	}
	if !input.HasPreferences {
		reasons = append(reasons, ReadinessMissingPreferences)
	}
	if input.SelectedBoardID == uuid.Nil {
		reasons = append(reasons, ReadinessMissingSelectedBoard)
	}
	return Readiness{Ready: len(reasons) == 0, Reasons: reasons}
}

func validateNormalizedText(value string, maximumRunes int) error {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > maximumRunes {
		return errors.New("text must be non-empty, normalized, and within its length limit")
	}
	return nil
}

func validateRestoredAggregate(version int64, createdAt, updatedAt time.Time) error {
	if version <= 0 {
		return errors.New("aggregate version must be positive")
	}
	if createdAt.IsZero() || updatedAt.IsZero() {
		return errors.New("aggregate timestamps are required")
	}
	if updatedAt.Before(createdAt) {
		return errors.New("aggregate update time precedes creation time")
	}
	return nil
}

// FiniteDecimalToInteger quantizes an adapter-supplied decimal into canonical
// integer units using round-half-away-from-zero. Adapters provide the scale;
// domain aggregates store only the returned integer.
func FiniteDecimalToInteger(value, scale float64) (int, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
		return 0, errors.New("quantization requires finite value and positive finite scale")
	}
	quantized := math.Round(value * scale)
	if quantized > float64(math.MaxInt) || quantized < float64(math.MinInt) {
		return 0, errors.New("quantized value exceeds integer range")
	}
	return int(quantized), nil
}
