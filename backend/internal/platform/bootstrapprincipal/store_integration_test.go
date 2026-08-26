//go:build integration

package bootstrapprincipal_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/platform/bootstrapprincipal"
)

func TestPostgresProvisionerIsIdempotentAndRejectsDisabledPrincipal(t *testing.T) {
	databaseURL := os.Getenv("TEST_MIGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_MIGRATION_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect migration database: %v", err)
	}
	defer connection.Close(context.Background())

	principalUUID := uuid.New()
	principalID, err := identity.ParsePrincipalID(principalUUID.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = connection.Exec(cleanupCtx, "DELETE FROM principals WHERE id = $1", principalUUID)
	})

	provisioner := bootstrapprincipal.NewPostgresProvisioner(connection)
	created, err := provisioner.Ensure(ctx, principalID)
	if err != nil || !created {
		t.Fatalf("first Ensure() = (%t, %v), want (true, nil)", created, err)
	}
	created, err = provisioner.Ensure(ctx, principalID)
	if err != nil || created {
		t.Fatalf("second Ensure() = (%t, %v), want (false, nil)", created, err)
	}

	if _, err := connection.Exec(ctx, "UPDATE principals SET disabled_at = transaction_timestamp() WHERE id = $1", principalUUID); err != nil {
		t.Fatalf("disable principal: %v", err)
	}
	if _, err := provisioner.Ensure(ctx, principalID); !errors.Is(err, bootstrapprincipal.ErrPrincipalDisabled) {
		t.Fatalf("Ensure(disabled) error = %v", err)
	}
}
