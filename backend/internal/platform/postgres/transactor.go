// Package postgres implements user-data persistence with pgx and transaction-
// local PostgreSQL row-level-security context.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonriber/the-search-surf/backend/internal/application/userdata"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

// Transactor binds repositories and trusted principal scope to one pgx transaction.
type Transactor struct {
	pool *pgxpool.Pool
}

// NewTransactor creates a user-data transaction adapter.
func NewTransactor(pool *pgxpool.Pool) (*Transactor, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &Transactor{pool: pool}, nil
}

// WithinTransaction owns begin, principal scoping, commit, and rollback.
func (transactor *Transactor) WithinTransaction(ctx context.Context, principal identity.PrincipalID, operation func(context.Context, userdata.Transaction) error) error {
	if principal.IsZero() {
		return errors.New("trusted principal is required")
	}
	if operation == nil {
		return errors.New("transaction operation is required")
	}

	tx, err := transactor.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin user-data transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.principal_id', $1, true)", principal.String()); err != nil {
		return errors.Join(fmt.Errorf("scope user-data transaction: %w", err), rollback(ctx, tx))
	}

	transaction := stores{profiles: profileRepository{tx: tx}}
	if err := operation(ctx, transaction); err != nil {
		return errors.Join(err, rollback(ctx, tx))
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user-data transaction: %w", err)
	}
	return nil
}

type stores struct {
	profiles profileRepository
}

func (transaction stores) Profiles() userdata.ProfileRepository {
	return transaction.profiles
}

func rollback(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("rollback user-data transaction: %w", err)
	}
	return nil
}
