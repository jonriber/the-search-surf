package userdata

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/spots"
)

type spotRepositoryStub struct {
	created         spots.Spot
	createResult    spots.Spot
	createErr       error
	getOwner        identity.PrincipalID
	getID           uuid.UUID
	getResult       spots.Spot
	getErr          error
	listOwner       identity.PrincipalID
	listResult      []spots.Spot
	listErr         error
	updated         spots.Spot
	expectedVersion int64
	updateResult    spots.Spot
	updateErr       error
	deletedOwner    identity.PrincipalID
	deletedID       uuid.UUID
	deleteVersion   int64
	deleteErr       error
}

func (stub *spotRepositoryStub) Create(_ context.Context, candidate spots.Spot) (spots.Spot, error) {
	stub.created = candidate
	return stub.createResult, stub.createErr
}
func (stub *spotRepositoryStub) Get(_ context.Context, owner identity.PrincipalID, id uuid.UUID) (spots.Spot, error) {
	stub.getOwner, stub.getID = owner, id
	return stub.getResult, stub.getErr
}
func (stub *spotRepositoryStub) List(_ context.Context, owner identity.PrincipalID) ([]spots.Spot, error) {
	stub.listOwner = owner
	return stub.listResult, stub.listErr
}
func (stub *spotRepositoryStub) Update(_ context.Context, candidate spots.Spot, expectedVersion int64) (spots.Spot, error) {
	stub.updated, stub.expectedVersion = candidate, expectedVersion
	return stub.updateResult, stub.updateErr
}
func (stub *spotRepositoryStub) Delete(_ context.Context, owner identity.PrincipalID, id uuid.UUID, expectedVersion int64) error {
	stub.deletedOwner, stub.deletedID, stub.deleteVersion = owner, id, expectedVersion
	return stub.deleteErr
}

type favoriteRepositoryStub struct {
	added           spots.Favorite
	addResult       spots.Favorite
	addErr          error
	listOwner       identity.PrincipalID
	listResult      []spots.Favorite
	listErr         error
	updatedOwner    identity.PrincipalID
	updatedSpotID   uuid.UUID
	updatedPosition int
	updateResult    spots.Favorite
	updateErr       error
	removedOwner    identity.PrincipalID
	removedSpotID   uuid.UUID
	removeErr       error
}

func (stub *favoriteRepositoryStub) Add(_ context.Context, favorite spots.Favorite) (spots.Favorite, error) {
	stub.added = favorite
	return stub.addResult, stub.addErr
}
func (stub *favoriteRepositoryStub) List(_ context.Context, owner identity.PrincipalID) ([]spots.Favorite, error) {
	stub.listOwner = owner
	return stub.listResult, stub.listErr
}
func (stub *favoriteRepositoryStub) UpdatePosition(_ context.Context, owner identity.PrincipalID, spotID uuid.UUID, position int) (spots.Favorite, error) {
	stub.updatedOwner, stub.updatedSpotID, stub.updatedPosition = owner, spotID, position
	return stub.updateResult, stub.updateErr
}
func (stub *favoriteRepositoryStub) Remove(_ context.Context, owner identity.PrincipalID, spotID uuid.UUID) error {
	stub.removedOwner, stub.removedSpotID = owner, spotID
	return stub.removeErr
}

func TestSpotUseCasesExercisePermittedLifecycle(t *testing.T) {
	t.Parallel()

	principal := mustPrincipalID(t)
	spotID := uuid.MustParse("4fda51a7-d38b-47b2-8c84-aaf455a73602")
	want, err := spots.NewSpot(spotID, principal, "Supertubos", -9.3645, 39.3394, "Europe/Lisbon")
	if err != nil {
		t.Fatal(err)
	}
	updated := want
	updated.Name, updated.Version = "Supertubos North", 2
	repository := &spotRepositoryStub{createResult: want, getResult: want, listResult: []spots.Spot{want}, updateResult: updated}
	transactions := &transactorStub{transaction: transactionStub{spots: repository}}
	service, err := NewService(transactions, func() uuid.UUID { return spotID })
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateSpot(context.Background(), principal, SpotInput{Name: "Supertubos", Longitude: -9.3645, Latitude: 39.3394, TimeZone: "Europe/Lisbon"})
	if err != nil || created != want || repository.created.OwnerID != principal || repository.created.ID != spotID {
		t.Fatalf("CreateSpot() = (%+v, %v)", created, err)
	}
	got, err := service.GetSpot(context.Background(), principal, spotID)
	if err != nil || got != want || repository.getOwner != principal {
		t.Fatalf("GetSpot() = (%+v, %v)", got, err)
	}
	listed, err := service.ListSpots(context.Background(), principal)
	if err != nil || len(listed) != 1 || repository.listOwner != principal {
		t.Fatalf("ListSpots() = (%+v, %v)", listed, err)
	}
	gotUpdate, err := service.UpdateSpot(context.Background(), principal, spotID, UpdateSpotInput{SpotInput: SpotInput{Name: "Supertubos North", Longitude: -9.36, Latitude: 39.34, TimeZone: "Europe/Lisbon"}, ExpectedVersion: 1})
	if err != nil || gotUpdate.Version != 2 || repository.updated.ID != spotID || repository.expectedVersion != 1 {
		t.Fatalf("UpdateSpot() = (%+v, %v)", gotUpdate, err)
	}
	if err := service.DeleteSpot(context.Background(), principal, spotID, 2); err != nil || repository.deletedOwner != principal || repository.deleteVersion != 2 {
		t.Fatalf("DeleteSpot() error = %v", err)
	}
}

func TestFavoriteUseCasesExercisePermittedLifecycle(t *testing.T) {
	t.Parallel()

	principal := mustPrincipalID(t)
	spotID := uuid.MustParse("4fda51a7-d38b-47b2-8c84-aaf455a73602")
	want, err := spots.NewFavorite(principal, spotID, 3)
	if err != nil {
		t.Fatal(err)
	}
	reordered := want
	reordered.SortPosition = 1
	repository := &favoriteRepositoryStub{addResult: want, listResult: []spots.Favorite{want}, updateResult: reordered}
	transactions := &transactorStub{transaction: transactionStub{favorites: repository}}
	service, err := NewService(transactions, uuid.New)
	if err != nil {
		t.Fatal(err)
	}

	added, err := service.AddFavorite(context.Background(), principal, spotID, 3)
	if err != nil || added != want || repository.added.OwnerID != principal {
		t.Fatalf("AddFavorite() = (%+v, %v)", added, err)
	}
	listed, err := service.ListFavorites(context.Background(), principal)
	if err != nil || len(listed) != 1 || repository.listOwner != principal {
		t.Fatalf("ListFavorites() = (%+v, %v)", listed, err)
	}
	gotUpdate, err := service.UpdateFavoritePosition(context.Background(), principal, spotID, 1)
	if err != nil || gotUpdate.SortPosition != 1 || repository.updatedPosition != 1 {
		t.Fatalf("UpdateFavoritePosition() = (%+v, %v)", gotUpdate, err)
	}
	if err := service.RemoveFavorite(context.Background(), principal, spotID); err != nil || repository.removedOwner != principal {
		t.Fatalf("RemoveFavorite() error = %v", err)
	}
}

func TestSpotAndFavoriteValidationFailsBeforeTransaction(t *testing.T) {
	t.Parallel()

	transactions := &transactorStub{}
	service, err := NewService(transactions, func() uuid.UUID { return uuid.Nil })
	if err != nil {
		t.Fatal(err)
	}
	principal := mustPrincipalID(t)
	spotID := uuid.MustParse("4fda51a7-d38b-47b2-8c84-aaf455a73602")

	operations := []func() error{
		func() error {
			_, err := service.CreateSpot(context.Background(), principal, SpotInput{Name: "Spot", TimeZone: "UTC"})
			return err
		},
		func() error {
			_, err := service.UpdateSpot(context.Background(), principal, spotID, UpdateSpotInput{ExpectedVersion: 0})
			return err
		},
		func() error { return service.DeleteSpot(context.Background(), principal, spotID, 0) },
		func() error { _, err := service.AddFavorite(context.Background(), principal, spotID, -1); return err },
		func() error {
			_, err := service.UpdateFavoritePosition(context.Background(), principal, spotID, -1)
			return err
		},
		func() error { return service.RemoveFavorite(context.Background(), principal, uuid.Nil) },
	}
	for index, operation := range operations {
		if err := operation(); err == nil {
			t.Fatalf("operation %d error = nil", index)
		}
	}
	if transactions.calls != 0 {
		t.Fatalf("transaction calls = %d, want 0", transactions.calls)
	}
}

func TestSpotRepositoryFailureIsPreserved(t *testing.T) {
	t.Parallel()

	want := errors.New("spot unavailable")
	service, err := NewService(&transactorStub{transaction: transactionStub{spots: &spotRepositoryStub{listErr: want}}}, uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListSpots(context.Background(), mustPrincipalID(t)); !errors.Is(err, want) {
		t.Fatalf("ListSpots() error = %v", err)
	}
}
