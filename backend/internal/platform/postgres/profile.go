package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jonriber/the-search-surf/backend/internal/application/userdata"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/profile"
)

type profileRepository struct {
	tx pgx.Tx
}

func (repository profileRepository) Create(ctx context.Context, candidate profile.Profile) (profile.Profile, error) {
	row := repository.tx.QueryRow(ctx, `
		INSERT INTO surfer_profiles (owner_id, experience_level, display_units)
		VALUES ($1, $2, $3)
		RETURNING owner_id, experience_level, display_units, version, created_at, updated_at
	`, candidate.OwnerID.UUID(), candidate.ExperienceLevel, candidate.DisplayUnits)
	created, err := scanProfile(row)
	if err != nil {
		if isUniqueViolation(err) {
			return profile.Profile{}, userdata.ErrAlreadyExists
		}
		return profile.Profile{}, fmt.Errorf("insert profile: %w", err)
	}
	return created, nil
}

func (repository profileRepository) Get(ctx context.Context, owner identity.PrincipalID) (profile.Profile, error) {
	row := repository.tx.QueryRow(ctx, `
		SELECT owner_id, experience_level, display_units, version, created_at, updated_at
		FROM surfer_profiles
		WHERE owner_id = $1
	`, owner.UUID())
	found, err := scanProfile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.Profile{}, userdata.ErrNotFound
	}
	if err != nil {
		return profile.Profile{}, fmt.Errorf("select profile: %w", err)
	}
	return found, nil
}

func (repository profileRepository) Update(ctx context.Context, candidate profile.Profile, expectedVersion int64) (profile.Profile, error) {
	row := repository.tx.QueryRow(ctx, `
		UPDATE surfer_profiles
		SET experience_level = $2,
			display_units = $3,
			version = version + 1,
			updated_at = transaction_timestamp()
		WHERE owner_id = $1 AND version = $4
		RETURNING owner_id, experience_level, display_units, version, created_at, updated_at
	`, candidate.OwnerID.UUID(), candidate.ExperienceLevel, candidate.DisplayUnits, expectedVersion)
	updated, err := scanProfile(row)
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return profile.Profile{}, fmt.Errorf("update profile: %w", err)
		}
		return updated, nil
	}

	var exists bool
	if err := repository.tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM surfer_profiles WHERE owner_id = $1)", candidate.OwnerID.UUID()).Scan(&exists); err != nil {
		return profile.Profile{}, fmt.Errorf("resolve profile update outcome: %w", err)
	}
	if exists {
		return profile.Profile{}, userdata.ErrConflict
	}
	return profile.Profile{}, userdata.ErrNotFound
}

type rowScanner interface {
	Scan(...any) error
}

func scanProfile(row rowScanner) (profile.Profile, error) {
	var ownerUUID uuid.UUID
	var experience, units string
	var version int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&ownerUUID, &experience, &units, &version, &createdAt, &updatedAt); err != nil {
		return profile.Profile{}, err
	}
	owner, err := identity.ParsePrincipalID(ownerUUID.String())
	if err != nil {
		return profile.Profile{}, fmt.Errorf("restore profile owner: %w", err)
	}
	restored, err := profile.Restore(owner, experience, units, version, createdAt, updatedAt)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("restore profile: %w", err)
	}
	return restored, nil
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
