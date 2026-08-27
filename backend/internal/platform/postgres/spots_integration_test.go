//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonriber/the-search-surf/backend/internal/application/userdata"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/platform/postgres"
)

func TestSpotAndFavoritePersistenceContracts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	migrationConnection, err := pgx.Connect(ctx, requiredEnvironment(t, "TEST_MIGRATION_DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect migration database: %v", err)
	}
	defer migrationConnection.Close(context.Background())

	principalA, principalB := newPrincipalID(t), newPrincipalID(t)
	for _, principal := range []identity.PrincipalID{principalA, principalB} {
		if _, err := migrationConnection.Exec(ctx, "INSERT INTO principals (id) VALUES ($1)", principal.UUID()); err != nil {
			t.Fatalf("seed principal: %v", err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, principal := range []identity.PrincipalID{principalA, principalB} {
			_, _ = migrationConnection.Exec(cleanupCtx, "DELETE FROM principals WHERE id = $1", principal.UUID())
		}
	})

	pool, err := pgxpool.New(ctx, requiredEnvironment(t, "TEST_APPLICATION_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create application pool: %v", err)
	}
	defer pool.Close()
	transactions, err := postgres.NewTransactor(pool)
	if err != nil {
		t.Fatal(err)
	}

	spotA1 := uuid.MustParse("a0000000-0000-4000-8000-000000000001")
	spotA2 := uuid.MustParse("a0000000-0000-4000-8000-000000000002")
	spotB := uuid.MustParse("b0000000-0000-4000-8000-000000000001")
	ids := []uuid.UUID{spotA1, spotA2}
	serviceA, err := userdata.NewService(transactions, func() uuid.UUID {
		id := ids[0]
		ids = ids[1:]
		return id
	})
	if err != nil {
		t.Fatal(err)
	}
	serviceB, err := userdata.NewService(transactions, func() uuid.UUID { return spotB })
	if err != nil {
		t.Fatal(err)
	}

	createdA1, err := serviceA.CreateSpot(ctx, principalA, userdata.SpotInput{Name: "Z spot", Longitude: -9.3645, Latitude: 39.3394, TimeZone: "Europe/Lisbon"})
	if err != nil || createdA1.ID != spotA1 || createdA1.Version != 1 {
		t.Fatalf("CreateSpot(A1) = (%+v, %v)", createdA1, err)
	}
	createdA2, err := serviceA.CreateSpot(ctx, principalA, userdata.SpotInput{Name: "A spot", Longitude: -8.9, Latitude: 38.7, TimeZone: "Europe/Lisbon"})
	if err != nil || createdA2.ID != spotA2 {
		t.Fatalf("CreateSpot(A2) = (%+v, %v)", createdA2, err)
	}
	createdB, err := serviceB.CreateSpot(ctx, principalB, userdata.SpotInput{Name: "B spot", Longitude: -7.0, Latitude: 37.0, TimeZone: "UTC"})
	if err != nil {
		t.Fatalf("CreateSpot(B) error = %v", err)
	}

	listed, err := serviceA.ListSpots(ctx, principalA)
	if err != nil || len(listed) != 2 || listed[0].ID != spotA2 || listed[1].ID != spotA1 {
		t.Fatalf("ListSpots() = (%+v, %v)", listed, err)
	}
	readA1, err := serviceA.GetSpot(ctx, principalA, spotA1)
	if err != nil || readA1.Longitude != -9.3645 || readA1.Latitude != 39.3394 {
		t.Fatalf("GetSpot() = (%+v, %v)", readA1, err)
	}
	if _, err := serviceA.GetSpot(ctx, principalA, createdB.ID); !errors.Is(err, userdata.ErrNotFound) {
		t.Fatalf("cross-owner GetSpot() error = %v", err)
	}

	updatedA1, err := serviceA.UpdateSpot(ctx, principalA, spotA1, userdata.UpdateSpotInput{SpotInput: userdata.SpotInput{Name: "Z spot updated", Longitude: -9.3, Latitude: 39.3, TimeZone: "Europe/Lisbon"}, ExpectedVersion: 1})
	if err != nil || updatedA1.Version != 2 {
		t.Fatalf("UpdateSpot() = (%+v, %v)", updatedA1, err)
	}
	if _, err := serviceA.UpdateSpot(ctx, principalA, spotA1, userdata.UpdateSpotInput{SpotInput: userdata.SpotInput{Name: "stale", TimeZone: "UTC"}, ExpectedVersion: 1}); !errors.Is(err, userdata.ErrConflict) {
		t.Fatalf("stale UpdateSpot() error = %v", err)
	}

	if _, err := serviceA.AddFavorite(ctx, principalA, createdB.ID, 0); !errors.Is(err, userdata.ErrNotFound) {
		t.Fatalf("cross-owner AddFavorite() error = %v", err)
	}
	if _, err := serviceA.AddFavorite(ctx, principalA, spotA1, 5); err != nil {
		t.Fatalf("AddFavorite(A1) error = %v", err)
	}
	if _, err := serviceA.AddFavorite(ctx, principalA, spotA2, 1); err != nil {
		t.Fatalf("AddFavorite(A2) error = %v", err)
	}
	if _, err := serviceA.AddFavorite(ctx, principalA, spotA2, 1); !errors.Is(err, userdata.ErrAlreadyExists) {
		t.Fatalf("duplicate AddFavorite() error = %v", err)
	}
	favorites, err := serviceA.ListFavorites(ctx, principalA)
	if err != nil || len(favorites) != 2 || favorites[0].SpotID != spotA2 || favorites[1].SpotID != spotA1 {
		t.Fatalf("ListFavorites() = (%+v, %v)", favorites, err)
	}
	if _, err := serviceA.UpdateFavoritePosition(ctx, principalA, spotA1, 0); err != nil {
		t.Fatalf("UpdateFavoritePosition() error = %v", err)
	}
	favorites, err = serviceA.ListFavorites(ctx, principalA)
	if err != nil || favorites[0].SpotID != spotA1 {
		t.Fatalf("reordered ListFavorites() = (%+v, %v)", favorites, err)
	}
	if err := serviceA.RemoveFavorite(ctx, principalA, spotA1); err != nil {
		t.Fatalf("RemoveFavorite() error = %v", err)
	}
	if _, err := serviceA.GetSpot(ctx, principalA, spotA1); err != nil {
		t.Fatalf("spot removed with favorite: %v", err)
	}
	if err := serviceA.RemoveFavorite(ctx, principalA, spotA1); !errors.Is(err, userdata.ErrNotFound) {
		t.Fatalf("second RemoveFavorite() error = %v", err)
	}

	if err := serviceA.DeleteSpot(ctx, principalA, spotA1, 1); !errors.Is(err, userdata.ErrConflict) {
		t.Fatalf("stale DeleteSpot() error = %v", err)
	}
	if err := serviceA.DeleteSpot(ctx, principalA, spotA1, 2); err != nil {
		t.Fatalf("DeleteSpot() error = %v", err)
	}
	if _, err := serviceA.GetSpot(ctx, principalA, spotA1); !errors.Is(err, userdata.ErrNotFound) {
		t.Fatalf("deleted GetSpot() error = %v", err)
	}

	duplicateService, err := userdata.NewService(transactions, func() uuid.UUID { return spotA2 })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := duplicateService.CreateSpot(ctx, principalA, userdata.SpotInput{Name: "duplicate", TimeZone: "UTC"}); !errors.Is(err, userdata.ErrAlreadyExists) {
		t.Fatalf("duplicate CreateSpot() error = %v", err)
	}
}
