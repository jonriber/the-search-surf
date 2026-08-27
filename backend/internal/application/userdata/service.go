// Package userdata implements ownership-scoped profile, spot, and favorite use
// cases without depending on HTTP or PostgreSQL.
package userdata

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/profile"
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

// Transaction exposes repositories bound to one database transaction.
type Transaction interface {
	Profiles() ProfileRepository
}

// Transactor owns begin, trusted principal scope, commit, and rollback.
type Transactor interface {
	WithinTransaction(context.Context, identity.PrincipalID, func(context.Context, Transaction) error) error
}

// Service executes user-data use cases through explicit transaction boundaries.
type Service struct {
	transactions Transactor
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

// NewService constructs the user-data application service.
func NewService(transactions Transactor) (*Service, error) {
	if transactions == nil {
		return nil, errors.New("user-data transactor is required")
	}
	return &Service{transactions: transactions}, nil
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
