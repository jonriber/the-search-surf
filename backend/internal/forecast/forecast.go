// Package forecast defines provider-neutral forecast values and provenance.
package forecast

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

const maxPointReferenceLength = 200

// Point identifies an application-owned forecast sampling point. Reference is
// used to correlate batched responses without comparing floating-point values.
type Point struct {
	Reference string
	Longitude float64
	Latitude  float64
}

// NewPoint validates an application-owned sampling point.
func NewPoint(reference string, longitude, latitude float64) (Point, error) {
	normalizedReference := strings.TrimSpace(reference)
	point := Point{Reference: normalizedReference, Longitude: longitude, Latitude: latitude}
	if err := point.Validate(); err != nil {
		return Point{}, err
	}
	return point, nil
}

// Validate checks a point even when it did not come from NewPoint.
func (point Point) Validate() error {
	if point.Reference == "" || point.Reference != strings.TrimSpace(point.Reference) || utf8.RuneCountInString(point.Reference) > maxPointReferenceLength || strings.ContainsRune(point.Reference, '\x00') {
		return errors.New("forecast point reference must contain between 1 and 200 normalized characters")
	}
	if math.IsNaN(point.Longitude) || math.IsInf(point.Longitude, 0) || point.Longitude < -180 || point.Longitude > 180 {
		return errors.New("forecast point longitude must be between -180 and 180")
	}
	if math.IsNaN(point.Latitude) || math.IsInf(point.Latitude, 0) || point.Latitude < -90 || point.Latitude > 90 {
		return errors.New("forecast point latitude must be between -90 and 90")
	}
	return nil
}

// Measurement makes missing provider data distinct from a measured zero.
type Measurement struct {
	Value     float64
	Available bool
}

// NewMeasurement constructs an available finite measurement.
func NewMeasurement(value float64) (Measurement, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Measurement{}, errors.New("forecast measurement must be finite")
	}
	return Measurement{Value: value, Available: true}, nil
}

// MissingMeasurement constructs an explicitly unavailable measurement.
func MissingMeasurement() Measurement {
	return Measurement{}
}

// HourlyConditions contains canonical SI measurements for one UTC instant.
// Provider adapters reject invalid physical ranges before constructing a batch.
type HourlyConditions struct {
	ValidAt                      time.Time
	WaveHeightMetres             Measurement
	WaveDirectionDegrees         Measurement
	WavePeriodSeconds            Measurement
	SwellHeightMetres            Measurement
	SwellDirectionDegrees        Measurement
	SwellPeriodSeconds           Measurement
	WindSpeedMetresPerSecond     Measurement
	WindDirectionDegrees         Measurement
	WindGustMetresPerSecond      Measurement
	SeaLevelHeightMetresAboveMSL Measurement
}

// Component identifies the canonical part of a forecast supplied by a source
// model. It is deliberately independent of provider field names.
type Component string

// Supported forecast source components.
const (
	ComponentWaves    Component = "waves"
	ComponentWind     Component = "wind"
	ComponentSeaLevel Component = "sea_level"
)

// Source identifies the opaque upstream model reference used for one
// component. IssueTimeKnown is false only when the provider cannot expose it;
// callers must never substitute the fetch time.
type Source struct {
	Component                Component
	ModelReference           string
	IssuedAt                 time.Time
	AvailableAt              time.Time
	IssueTimeKnown           bool
	SampledLongitude         float64
	SampledLatitude          float64
	NativeTemporalResolution time.Duration
}

// Attribution describes credit that must travel with displayed or
// redistributed forecast data.
type Attribution struct {
	Text       string
	URL        string
	License    string
	LicenseURL string
}

// PayloadDigest identifies a retained raw provider payload without putting the
// payload or private coordinates in the domain value.
type PayloadDigest struct {
	Component Component
	SHA256    string
}

// Series contains normalized conditions for one requested point. Sampled
// coordinates may differ from the requested point because providers use grids.
type Series struct {
	PointReference string
	Sources        []Source
	Hours          []HourlyConditions
}

// Batch is the provider-neutral result of one logical provider fetch.
type Batch struct {
	ProviderID            string
	FetchedAt             time.Time
	TransformationVersion string
	Attribution           Attribution
	PayloadDigests        []PayloadDigest
	Series                []Series
}
