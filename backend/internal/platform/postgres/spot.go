package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jonriber/the-search-surf/backend/internal/application/userdata"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/spots"
)

type spotRepository struct {
	tx pgx.Tx
}

func (repository spotRepository) Create(ctx context.Context, candidate spots.Spot) (spots.Spot, error) {
	row := repository.tx.QueryRow(ctx, `
		INSERT INTO surf_spots (id, owner_id, name, position, time_zone)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6)
		RETURNING id, owner_id, name,
			ST_X(position::geometry), ST_Y(position::geometry),
			time_zone, version, created_at, updated_at
	`, candidate.ID, candidate.OwnerID.UUID(), candidate.Name, candidate.Longitude, candidate.Latitude, candidate.TimeZone)
	created, err := scanSpot(row)
	if err != nil {
		if isUniqueViolation(err) {
			return spots.Spot{}, userdata.ErrAlreadyExists
		}
		return spots.Spot{}, fmt.Errorf("insert spot: %w", err)
	}
	return created, nil
}

func (repository spotRepository) Get(ctx context.Context, owner identity.PrincipalID, id uuid.UUID) (spots.Spot, error) {
	row := repository.tx.QueryRow(ctx, `
		SELECT id, owner_id, name,
			ST_X(position::geometry), ST_Y(position::geometry),
			time_zone, version, created_at, updated_at
		FROM surf_spots
		WHERE owner_id = $1 AND id = $2
	`, owner.UUID(), id)
	found, err := scanSpot(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return spots.Spot{}, userdata.ErrNotFound
	}
	if err != nil {
		return spots.Spot{}, fmt.Errorf("select spot: %w", err)
	}
	return found, nil
}

func (repository spotRepository) List(ctx context.Context, owner identity.PrincipalID) ([]spots.Spot, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT id, owner_id, name,
			ST_X(position::geometry), ST_Y(position::geometry),
			time_zone, version, created_at, updated_at
		FROM surf_spots
		WHERE owner_id = $1
		ORDER BY name, id
	`, owner.UUID())
	if err != nil {
		return nil, fmt.Errorf("list spots: %w", err)
	}
	defer rows.Close()

	found := make([]spots.Spot, 0)
	for rows.Next() {
		spot, err := scanSpot(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed spot: %w", err)
		}
		found = append(found, spot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spots: %w", err)
	}
	return found, nil
}

func (repository spotRepository) Update(ctx context.Context, candidate spots.Spot, expectedVersion int64) (spots.Spot, error) {
	row := repository.tx.QueryRow(ctx, `
		UPDATE surf_spots
		SET name = $3,
			position = ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography,
			time_zone = $6,
			version = version + 1,
			updated_at = transaction_timestamp()
		WHERE owner_id = $1 AND id = $2 AND version = $7
		RETURNING id, owner_id, name,
			ST_X(position::geometry), ST_Y(position::geometry),
			time_zone, version, created_at, updated_at
	`, candidate.OwnerID.UUID(), candidate.ID, candidate.Name, candidate.Longitude, candidate.Latitude, candidate.TimeZone, expectedVersion)
	updated, err := scanSpot(row)
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return spots.Spot{}, fmt.Errorf("update spot: %w", err)
		}
		return updated, nil
	}

	exists, err := repository.exists(ctx, candidate.OwnerID, candidate.ID)
	if err != nil {
		return spots.Spot{}, fmt.Errorf("resolve spot update outcome: %w", err)
	}
	if exists {
		return spots.Spot{}, userdata.ErrConflict
	}
	return spots.Spot{}, userdata.ErrNotFound
}

func (repository spotRepository) Delete(ctx context.Context, owner identity.PrincipalID, id uuid.UUID, expectedVersion int64) error {
	var deleted uuid.UUID
	err := repository.tx.QueryRow(ctx, `
		DELETE FROM surf_spots
		WHERE owner_id = $1 AND id = $2 AND version = $3
		RETURNING id
	`, owner.UUID(), id, expectedVersion).Scan(&deleted)
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return fmt.Errorf("delete spot: %w", err)
		}
		return nil
	}

	exists, err := repository.exists(ctx, owner, id)
	if err != nil {
		return fmt.Errorf("resolve spot delete outcome: %w", err)
	}
	if exists {
		return userdata.ErrConflict
	}
	return userdata.ErrNotFound
}

func (repository spotRepository) exists(ctx context.Context, owner identity.PrincipalID, id uuid.UUID) (bool, error) {
	var exists bool
	err := repository.tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM surf_spots WHERE owner_id = $1 AND id = $2
		)
	`, owner.UUID(), id).Scan(&exists)
	return exists, err
}

func scanSpot(row rowScanner) (spots.Spot, error) {
	var id, ownerUUID uuid.UUID
	var name, timeZone string
	var longitude, latitude float64
	var version int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &ownerUUID, &name, &longitude, &latitude, &timeZone, &version, &createdAt, &updatedAt); err != nil {
		return spots.Spot{}, err
	}
	owner, err := identity.ParsePrincipalID(ownerUUID.String())
	if err != nil {
		return spots.Spot{}, fmt.Errorf("restore spot owner: %w", err)
	}
	restored, err := spots.RestoreSpot(id, owner, name, longitude, latitude, timeZone, version, createdAt, updatedAt)
	if err != nil {
		return spots.Spot{}, fmt.Errorf("restore spot: %w", err)
	}
	return restored, nil
}
