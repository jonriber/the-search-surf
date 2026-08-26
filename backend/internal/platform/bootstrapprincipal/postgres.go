package bootstrapprincipal

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

// ErrPrincipalDisabled prevents a disabled environment identity from being
// silently reactivated during deployment.
var ErrPrincipalDisabled = errors.New("bootstrap principal is disabled")

// PostgresProvisioner stores bootstrap identities through a short-lived
// migration-role connection.
type PostgresProvisioner struct {
	connection *pgx.Conn
}

// NewPostgresProvisioner creates a PostgreSQL-backed provisioner.
func NewPostgresProvisioner(connection *pgx.Conn) *PostgresProvisioner {
	return &PostgresProvisioner{connection: connection}
}

// Ensure inserts a missing principal idempotently and rejects disabled rows.
func (provisioner *PostgresProvisioner) Ensure(ctx context.Context, principalID identity.PrincipalID) (bool, error) {
	if provisioner == nil || provisioner.connection == nil {
		return false, errors.New("PostgreSQL connection is required")
	}
	if principalID.IsZero() {
		return false, errors.New("principal ID is required")
	}

	tag, err := provisioner.connection.Exec(ctx, `
		INSERT INTO principals (id)
		VALUES ($1)
		ON CONFLICT (id) DO NOTHING
	`, principalID.UUID())
	if err != nil {
		return false, fmt.Errorf("insert bootstrap principal: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}

	var disabled bool
	if err := provisioner.connection.QueryRow(ctx, `
		SELECT disabled_at IS NOT NULL
		FROM principals
		WHERE id = $1
	`, principalID.UUID()).Scan(&disabled); err != nil {
		return false, fmt.Errorf("read bootstrap principal state: %w", err)
	}
	if disabled {
		return false, ErrPrincipalDisabled
	}
	return false, nil
}
