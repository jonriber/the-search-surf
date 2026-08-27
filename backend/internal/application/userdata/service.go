// Package userdata implements ownership-scoped profile, spot, and favorite use
// cases without depending on HTTP or PostgreSQL.
package userdata

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/profile"
	"github.com/jonriber/the-search-surf/backend/internal/spots"
)

// Stable application errors are translated by transport adapters without
// exposing persistence details.
var (
	ErrNotFound      = errors.New("user data not found")
	ErrConflict      = errors.New("user data version conflict")
	ErrAlreadyExists = errors.New("user data already exists")
)

// ProfileRepository is the capability-specific profile persistence port.
type ProfileRepository interface {
	Create(context.Context, profile.Profile) (profile.Profile, error)
	Get(context.Context, identity.PrincipalID) (profile.Profile, error)
	Update(context.Context, profile.Profile, int64) (profile.Profile, error)
}

// SpotRepository is the capability-specific private spot persistence port.
type SpotRepository interface {
	Create(context.Context, spots.Spot) (spots.Spot, error)
	Get(context.Context, identity.PrincipalID, uuid.UUID) (spots.Spot, error)
	List(context.Context, identity.PrincipalID) ([]spots.Spot, error)
	Update(context.Context, spots.Spot, int64) (spots.Spot, error)
	Delete(context.Context, identity.PrincipalID, uuid.UUID, int64) error
}

// FavoriteRepository is the owner-scoped favorite collection persistence port.
type FavoriteRepository interface {
	Add(context.Context, spots.Favorite) (spots.Favorite, error)
	List(context.Context, identity.PrincipalID) ([]spots.Favorite, error)
	UpdatePosition(context.Context, identity.PrincipalID, uuid.UUID, int) (spots.Favorite, error)
	Remove(context.Context, identity.PrincipalID, uuid.UUID) error
}

// Transaction exposes repositories bound to one database transaction.
type Transaction interface {
	Profiles() ProfileRepository
	Spots() SpotRepository
	Favorites() FavoriteRepository
}

// Transactor owns begin, trusted principal scope, commit, and rollback.
type Transactor interface {
	WithinTransaction(context.Context, identity.PrincipalID, func(context.Context, Transaction) error) error
}

// Service executes user-data use cases through explicit transaction boundaries.
type Service struct {
	transactions Transactor
	newSpotID    func() uuid.UUID
}

// ProfileInput contains mutable surfer profile attributes.
type ProfileInput struct {
	ExperienceLevel string
	DisplayUnits    string
}

// UpdateProfileInput adds optimistic concurrency to mutable profile input.
type UpdateProfileInput struct {
	ExperienceLevel string
	DisplayUnits    string
	ExpectedVersion int64
}

// SpotInput contains mutable private surf-spot attributes.
type SpotInput struct {
	Name      string
	Longitude float64
	Latitude  float64
	TimeZone  string
}

// UpdateSpotInput adds optimistic concurrency to mutable spot input.
type UpdateSpotInput struct {
	SpotInput
	ExpectedVersion int64
}

// NewService constructs the user-data application service.
func NewService(transactions Transactor, newSpotID func() uuid.UUID) (*Service, error) {
	if transactions == nil {
		return nil, errors.New("user-data transactor is required")
	}
	if newSpotID == nil {
		return nil, errors.New("spot ID generator is required")
	}
	return &Service{transactions: transactions, newSpotID: newSpotID}, nil
}

// CreateProfile creates the acting principal's one surfer profile.
func (service *Service) CreateProfile(ctx context.Context, principal identity.PrincipalID, input ProfileInput) (profile.Profile, error) {
	candidate, err := profile.New(principal, input.ExperienceLevel, input.DisplayUnits)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("validate profile: %w", err)
	}

	var created profile.Profile
	err = service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		created, err = transaction.Profiles().Create(ctx, candidate)
		return err
	})
	if err != nil {
		return profile.Profile{}, fmt.Errorf("create profile: %w", err)
	}
	return created, nil
}

// GetProfile returns only the acting principal's profile.
func (service *Service) GetProfile(ctx context.Context, principal identity.PrincipalID) (profile.Profile, error) {
	if principal.IsZero() {
		return profile.Profile{}, errors.New("trusted principal is required")
	}

	var found profile.Profile
	err := service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		var err error
		found, err = transaction.Profiles().Get(ctx, principal)
		return err
	})
	if err != nil {
		return profile.Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return found, nil
}

// UpdateProfile applies a compare-and-swap profile update.
func (service *Service) UpdateProfile(ctx context.Context, principal identity.PrincipalID, input UpdateProfileInput) (profile.Profile, error) {
	if input.ExpectedVersion <= 0 {
		return profile.Profile{}, errors.New("expected profile version must be positive")
	}
	candidate, err := profile.New(principal, input.ExperienceLevel, input.DisplayUnits)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("validate profile: %w", err)
	}

	var updated profile.Profile
	err = service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		updated, err = transaction.Profiles().Update(ctx, candidate, input.ExpectedVersion)
		return err
	})
	if err != nil {
		return profile.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return updated, nil
}

// CreateSpot creates a private spot with an application-generated identifier.
func (service *Service) CreateSpot(ctx context.Context, principal identity.PrincipalID, input SpotInput) (spots.Spot, error) {
	candidate, err := spots.NewSpot(service.newSpotID(), principal, input.Name, input.Longitude, input.Latitude, input.TimeZone)
	if err != nil {
		return spots.Spot{}, fmt.Errorf("validate spot: %w", err)
	}
	var created spots.Spot
	err = service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		created, err = transaction.Spots().Create(ctx, candidate)
		return err
	})
	if err != nil {
		return spots.Spot{}, fmt.Errorf("create spot: %w", err)
	}
	return created, nil
}

// GetSpot returns a private spot only within the acting owner's scope.
func (service *Service) GetSpot(ctx context.Context, principal identity.PrincipalID, spotID uuid.UUID) (spots.Spot, error) {
	if err := validatePrincipalAndSpotID(principal, spotID); err != nil {
		return spots.Spot{}, err
	}
	var found spots.Spot
	err := service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		var err error
		found, err = transaction.Spots().Get(ctx, principal, spotID)
		return err
	})
	if err != nil {
		return spots.Spot{}, fmt.Errorf("get spot: %w", err)
	}
	return found, nil
}

// ListSpots returns the acting owner's spots in repository-defined deterministic order.
func (service *Service) ListSpots(ctx context.Context, principal identity.PrincipalID) ([]spots.Spot, error) {
	if principal.IsZero() {
		return nil, errors.New("trusted principal is required")
	}
	var found []spots.Spot
	err := service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		var err error
		found, err = transaction.Spots().List(ctx, principal)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list spots: %w", err)
	}
	return found, nil
}

// UpdateSpot applies an ownership-scoped compare-and-swap update.
func (service *Service) UpdateSpot(ctx context.Context, principal identity.PrincipalID, spotID uuid.UUID, input UpdateSpotInput) (spots.Spot, error) {
	if input.ExpectedVersion <= 0 {
		return spots.Spot{}, errors.New("expected spot version must be positive")
	}
	candidate, err := spots.NewSpot(spotID, principal, input.Name, input.Longitude, input.Latitude, input.TimeZone)
	if err != nil {
		return spots.Spot{}, fmt.Errorf("validate spot: %w", err)
	}
	var updated spots.Spot
	err = service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		updated, err = transaction.Spots().Update(ctx, candidate, input.ExpectedVersion)
		return err
	})
	if err != nil {
		return spots.Spot{}, fmt.Errorf("update spot: %w", err)
	}
	return updated, nil
}

// DeleteSpot deletes a private spot only at the expected aggregate version.
func (service *Service) DeleteSpot(ctx context.Context, principal identity.PrincipalID, spotID uuid.UUID, expectedVersion int64) error {
	if err := validatePrincipalAndSpotID(principal, spotID); err != nil {
		return err
	}
	if expectedVersion <= 0 {
		return errors.New("expected spot version must be positive")
	}
	if err := service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		return transaction.Spots().Delete(ctx, principal, spotID, expectedVersion)
	}); err != nil {
		return fmt.Errorf("delete spot: %w", err)
	}
	return nil
}

// AddFavorite adds an owner-scoped spot relationship.
func (service *Service) AddFavorite(ctx context.Context, principal identity.PrincipalID, spotID uuid.UUID, sortPosition int) (spots.Favorite, error) {
	candidate, err := spots.NewFavorite(principal, spotID, sortPosition)
	if err != nil {
		return spots.Favorite{}, fmt.Errorf("validate favorite: %w", err)
	}
	var added spots.Favorite
	err = service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		added, err = transaction.Favorites().Add(ctx, candidate)
		return err
	})
	if err != nil {
		return spots.Favorite{}, fmt.Errorf("add favorite: %w", err)
	}
	return added, nil
}

// ListFavorites returns deterministic favorite ordering for the acting owner.
func (service *Service) ListFavorites(ctx context.Context, principal identity.PrincipalID) ([]spots.Favorite, error) {
	if principal.IsZero() {
		return nil, errors.New("trusted principal is required")
	}
	var found []spots.Favorite
	err := service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		var err error
		found, err = transaction.Favorites().List(ctx, principal)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	return found, nil
}

// UpdateFavoritePosition updates deterministic ordering input.
func (service *Service) UpdateFavoritePosition(ctx context.Context, principal identity.PrincipalID, spotID uuid.UUID, sortPosition int) (spots.Favorite, error) {
	if _, err := spots.NewFavorite(principal, spotID, sortPosition); err != nil {
		return spots.Favorite{}, fmt.Errorf("validate favorite: %w", err)
	}
	var updated spots.Favorite
	err := service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		var err error
		updated, err = transaction.Favorites().UpdatePosition(ctx, principal, spotID, sortPosition)
		return err
	})
	if err != nil {
		return spots.Favorite{}, fmt.Errorf("update favorite: %w", err)
	}
	return updated, nil
}

// RemoveFavorite removes the relationship while preserving the private spot.
func (service *Service) RemoveFavorite(ctx context.Context, principal identity.PrincipalID, spotID uuid.UUID) error {
	if err := validatePrincipalAndSpotID(principal, spotID); err != nil {
		return err
	}
	if err := service.transactions.WithinTransaction(ctx, principal, func(ctx context.Context, transaction Transaction) error {
		return transaction.Favorites().Remove(ctx, principal, spotID)
	}); err != nil {
		return fmt.Errorf("remove favorite: %w", err)
	}
	return nil
}

func validatePrincipalAndSpotID(principal identity.PrincipalID, spotID uuid.UUID) error {
	if principal.IsZero() {
		return errors.New("trusted principal is required")
	}
	if spotID == uuid.Nil {
		return errors.New("spot ID is required")
	}
	return nil
}
