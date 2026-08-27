//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonriber/the-search-surf/backend/internal/application/userdata"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/platform/postgres"
	"github.com/jonriber/the-search-surf/backend/internal/profile"
)

func TestProfilePersistenceAndTransactionContracts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	migrationURL := requiredEnvironment(t, "TEST_MIGRATION_DATABASE_URL")
	applicationURL := requiredEnvironment(t, "TEST_APPLICATION_DATABASE_URL")

	migrationConnection, err := pgx.Connect(ctx, migrationURL)
	if err != nil {
		t.Fatalf("connect migration database: %v", err)
	}
	defer migrationConnection.Close(context.Background())

	principalA := newPrincipalID(t)
	principalB := newPrincipalID(t)
	for _, principal := range []identity.PrincipalID{principalA, principalB} {
		if _, err := migrationConnection.Exec(ctx, "INSERT INTO principals (id) VALUES ($1)", principal.UUID()); err != nil {
			t.Fatalf("seed principal: %v", err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = migrationConnection.Exec(cleanupCtx, "DELETE FROM principals WHERE id = ANY($1)", []uuid.UUID{principalA.UUID(), principalB.UUID()})
	})

	pool, err := pgxpool.New(ctx, applicationURL)
	if err != nil {
		t.Fatalf("create application pool: %v", err)
	}
	defer pool.Close()
	transactions, err := postgres.NewTransactor(pool)
	if err != nil {
		t.Fatal(err)
	}
	service, err := userdata.NewService(transactions)
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateProfile(ctx, principalA, userdata.ProfileInput{ExperienceLevel: "intermediate", DisplayUnits: "metric"})
	if err != nil {
		t.Fatalf("CreateProfile() error = %v", err)
	}
	if created.OwnerID != principalA || created.Version != 1 || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created profile = %+v", created)
	}
	if _, err := service.CreateProfile(ctx, principalA, userdata.ProfileInput{ExperienceLevel: "advanced", DisplayUnits: "imperial"}); !errors.Is(err, userdata.ErrAlreadyExists) {
		t.Fatalf("duplicate CreateProfile() error = %v", err)
	}
	if _, err := service.GetProfile(ctx, principalB); !errors.Is(err, userdata.ErrNotFound) {
		t.Fatalf("cross-owner GetProfile() error = %v", err)
	}

	updated, err := service.UpdateProfile(ctx, principalA, userdata.UpdateProfileInput{ExperienceLevel: "advanced", DisplayUnits: "imperial", ExpectedVersion: 1})
	if err != nil || updated.Version != 2 || updated.ExperienceLevel != profile.ExperienceAdvanced {
		t.Fatalf("UpdateProfile() = (%+v, %v)", updated, err)
	}
	if _, err := service.UpdateProfile(ctx, principalA, userdata.UpdateProfileInput{ExperienceLevel: "expert", DisplayUnits: "metric", ExpectedVersion: 1}); !errors.Is(err, userdata.ErrConflict) {
		t.Fatalf("stale UpdateProfile() error = %v", err)
	}

	wantRollback := errors.New("force rollback")
	err = transactions.WithinTransaction(ctx, principalB, func(ctx context.Context, transaction userdata.Transaction) error {
		candidate, candidateErr := profile.New(principalB, "beginner", "metric")
		if candidateErr != nil {
			return candidateErr
		}
		if _, createErr := transaction.Profiles().Create(ctx, candidate); createErr != nil {
			return createErr
		}
		return wantRollback
	})
	if !errors.Is(err, wantRollback) {
		t.Fatalf("WithinTransaction() error = %v", err)
	}
	if _, err := service.GetProfile(ctx, principalB); !errors.Is(err, userdata.ErrNotFound) {
		t.Fatalf("rolled-back profile lookup error = %v", err)
	}
}

func requiredEnvironment(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func newPrincipalID(t *testing.T) identity.PrincipalID {
	t.Helper()
	id, err := identity.ParsePrincipalID(uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	return id
}
