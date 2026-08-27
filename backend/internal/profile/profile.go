// Package profile defines surfer-profile domain values and invariants.
package profile

import (
	"errors"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

// ExperienceLevel is the supported recommendation experience vocabulary.
type ExperienceLevel string

// Supported experience levels.
const (
	ExperienceBeginner     ExperienceLevel = "beginner"
	ExperienceIntermediate ExperienceLevel = "intermediate"
	ExperienceAdvanced     ExperienceLevel = "advanced"
	ExperienceExpert       ExperienceLevel = "expert"
)

// DisplayUnits controls presentation only; canonical values remain SI.
type DisplayUnits string

// Supported presentation unit systems.
const (
	UnitsMetric   DisplayUnits = "metric"
	UnitsImperial DisplayUnits = "imperial"
)

// Profile contains recommendation-relevant surfer characteristics.
type Profile struct {
	OwnerID         identity.PrincipalID
	ExperienceLevel ExperienceLevel
	DisplayUnits    DisplayUnits
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// New validates initial profile values.
func New(owner identity.PrincipalID, experience, units string) (Profile, error) {
	if owner.IsZero() {
		return Profile{}, errors.New("profile owner is required")
	}

	experienceLevel, err := parseExperienceLevel(experience)
	if err != nil {
		return Profile{}, err
	}
	displayUnits, err := parseDisplayUnits(units)
	if err != nil {
		return Profile{}, err
	}

	return Profile{
		OwnerID:         owner,
		ExperienceLevel: experienceLevel,
		DisplayUnits:    displayUnits,
		Version:         1,
	}, nil
}

// Restore validates profile state read from persistence.
func Restore(owner identity.PrincipalID, experience, units string, version int64, createdAt, updatedAt time.Time) (Profile, error) {
	restored, err := New(owner, experience, units)
	if err != nil {
		return Profile{}, err
	}
	if version <= 0 {
		return Profile{}, errors.New("profile version must be positive")
	}
	if updatedAt.Before(createdAt) {
		return Profile{}, errors.New("profile update time precedes creation time")
	}
	restored.Version = version
	restored.CreatedAt = createdAt
	restored.UpdatedAt = updatedAt
	return restored, nil
}

func parseExperienceLevel(value string) (ExperienceLevel, error) {
	experience := ExperienceLevel(value)
	switch experience {
	case ExperienceBeginner, ExperienceIntermediate, ExperienceAdvanced, ExperienceExpert:
		return experience, nil
	default:
		return "", errors.New("unsupported experience level")
	}
}

func parseDisplayUnits(value string) (DisplayUnits, error) {
	units := DisplayUnits(value)
	switch units {
	case UnitsMetric, UnitsImperial:
		return units, nil
	default:
		return "", errors.New("unsupported display units")
	}
}
