// Package spots defines private surf-spot and favorite domain invariants.
package spots

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/identity"

	_ "time/tzdata" // Embed IANA zones because the production API uses scratch.
)

const (
	maxSpotNameLength = 120
	maxTimeZoneLength = 255
)

// Spot is a private, user-owned representation of a surf break.
type Spot struct {
	ID        uuid.UUID
	OwnerID   identity.PrincipalID
	Name      string
	Longitude float64
	Latitude  float64
	TimeZone  string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Favorite is an owner-scoped relationship to a private surf spot.
type Favorite struct {
	OwnerID      identity.PrincipalID
	SpotID       uuid.UUID
	SortPosition int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewSpot validates and normalizes initial spot values.
func NewSpot(id uuid.UUID, owner identity.PrincipalID, name string, longitude, latitude float64, timeZone string) (Spot, error) {
	if id == uuid.Nil {
		return Spot{}, errors.New("spot ID is required")
	}
	if owner.IsZero() {
		return Spot{}, errors.New("spot owner is required")
	}

	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" || utf8.RuneCountInString(normalizedName) > maxSpotNameLength || strings.ContainsRune(normalizedName, '\x00') {
		return Spot{}, errors.New("spot name must contain between 1 and 120 characters")
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return Spot{}, errors.New("longitude must be between -180 and 180")
	}
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return Spot{}, errors.New("latitude must be between -90 and 90")
	}

	normalizedTimeZone := strings.TrimSpace(timeZone)
	if normalizedTimeZone == "" || utf8.RuneCountInString(normalizedTimeZone) > maxTimeZoneLength {
		return Spot{}, errors.New("time zone is required")
	}
	if _, err := time.LoadLocation(normalizedTimeZone); err != nil {
		return Spot{}, errors.New("time zone must be a recognized IANA identifier")
	}

	return Spot{
		ID:        id,
		OwnerID:   owner,
		Name:      normalizedName,
		Longitude: longitude,
		Latitude:  latitude,
		TimeZone:  normalizedTimeZone,
		Version:   1,
	}, nil
}

// NewFavorite validates an owner-scoped favorite relationship.
func NewFavorite(owner identity.PrincipalID, spotID uuid.UUID, sortPosition int) (Favorite, error) {
	if owner.IsZero() {
		return Favorite{}, errors.New("favorite owner is required")
	}
	if spotID == uuid.Nil {
		return Favorite{}, errors.New("favorite spot ID is required")
	}
	if sortPosition < 0 {
		return Favorite{}, errors.New("favorite sort position must not be negative")
	}
	return Favorite{OwnerID: owner, SpotID: spotID, SortPosition: sortPosition}, nil
}
