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
	"github.com/jonriber/the-search-surf/backend/internal/spots"
)

type favoriteRepository struct {
	tx pgx.Tx
}

func (repository favoriteRepository) Add(ctx context.Context, candidate spots.Favorite) (spots.Favorite, error) {
	row := repository.tx.QueryRow(ctx, `
		INSERT INTO favorites (owner_id, spot_id, sort_position)
		VALUES ($1, $2, $3)
		RETURNING owner_id, spot_id, sort_position, created_at, updated_at
	`, candidate.OwnerID.UUID(), candidate.SpotID, candidate.SortPosition)
	added, err := scanFavorite(row)
	if err != nil {
		switch {
		case isUniqueViolation(err):
			return spots.Favorite{}, userdata.ErrAlreadyExists
		case isForeignKeyViolation(err):
			return spots.Favorite{}, userdata.ErrNotFound
		default:
			return spots.Favorite{}, fmt.Errorf("insert favorite: %w", err)
		}
	}
	return added, nil
}

func (repository favoriteRepository) List(ctx context.Context, owner identity.PrincipalID) ([]spots.Favorite, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT owner_id, spot_id, sort_position, created_at, updated_at
		FROM favorites
		WHERE owner_id = $1
		ORDER BY sort_position, spot_id
	`, owner.UUID())
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	defer rows.Close()

	found := make([]spots.Favorite, 0)
	for rows.Next() {
		favorite, err := scanFavorite(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed favorite: %w", err)
		}
		found = append(found, favorite)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate favorites: %w", err)
	}
	return found, nil
}

func (repository favoriteRepository) UpdatePosition(ctx context.Context, owner identity.PrincipalID, spotID uuid.UUID, position int) (spots.Favorite, error) {
	row := repository.tx.QueryRow(ctx, `
		UPDATE favorites
		SET sort_position = $3,
			updated_at = transaction_timestamp()
		WHERE owner_id = $1 AND spot_id = $2
		RETURNING owner_id, spot_id, sort_position, created_at, updated_at
	`, owner.UUID(), spotID, position)
	updated, err := scanFavorite(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return spots.Favorite{}, userdata.ErrNotFound
	}
	if err != nil {
		return spots.Favorite{}, fmt.Errorf("update favorite: %w", err)
	}
	return updated, nil
}

func (repository favoriteRepository) Remove(ctx context.Context, owner identity.PrincipalID, spotID uuid.UUID) error {
	var removed uuid.UUID
	err := repository.tx.QueryRow(ctx, `
		DELETE FROM favorites
		WHERE owner_id = $1 AND spot_id = $2
		RETURNING spot_id
	`, owner.UUID(), spotID).Scan(&removed)
	if errors.Is(err, pgx.ErrNoRows) {
		return userdata.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete favorite: %w", err)
	}
	return nil
}

func scanFavorite(row rowScanner) (spots.Favorite, error) {
	var ownerUUID, spotID uuid.UUID
	var sortPosition int
	var createdAt, updatedAt time.Time
	if err := row.Scan(&ownerUUID, &spotID, &sortPosition, &createdAt, &updatedAt); err != nil {
		return spots.Favorite{}, err
	}
	owner, err := identity.ParsePrincipalID(ownerUUID.String())
	if err != nil {
		return spots.Favorite{}, fmt.Errorf("restore favorite owner: %w", err)
	}
	restored, err := spots.RestoreFavorite(owner, spotID, sortPosition, createdAt, updatedAt)
	if err != nil {
		return spots.Favorite{}, fmt.Errorf("restore favorite: %w", err)
	}
	return restored, nil
}

func isForeignKeyViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23503"
}
