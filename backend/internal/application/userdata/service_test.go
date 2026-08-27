package userdata

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/profile"
)

type profileRepositoryStub struct {
	created         profile.Profile
	createResult    profile.Profile
	createErr       error
	getOwner        identity.PrincipalID
	getResult       profile.Profile
	getErr          error
	updateCandidate profile.Profile
	expectedVersion int64
	updateResult    profile.Profile
	updateErr       error
}

func (stub *profileRepositoryStub) Create(_ context.Context, candidate profile.Profile) (profile.Profile, error) {
	stub.created = candidate
	return stub.createResult, stub.createErr
}

func (stub *profileRepositoryStub) Get(_ context.Context, owner identity.PrincipalID) (profile.Profile, error) {
	stub.getOwner = owner
	return stub.getResult, stub.getErr
}

func (stub *profileRepositoryStub) Update(_ context.Context, candidate profile.Profile, expectedVersion int64) (profile.Profile, error) {
	stub.updateCandidate = candidate
	stub.expectedVersion = expectedVersion
	return stub.updateResult, stub.updateErr
}

type transactionStub struct {
	profiles  ProfileRepository
	spots     SpotRepository
	favorites FavoriteRepository
}

func (stub transactionStub) Profiles() ProfileRepository   { return stub.profiles }
func (stub transactionStub) Spots() SpotRepository         { return stub.spots }
func (stub transactionStub) Favorites() FavoriteRepository { return stub.favorites }

type transactorStub struct {
	transaction Transaction
	err         error
	calls       int
	principal   identity.PrincipalID
}

func (stub *transactorStub) WithinTransaction(ctx context.Context, principal identity.PrincipalID, operation func(context.Context, Transaction) error) error {
	stub.calls++
	stub.principal = principal
	if stub.err != nil {
		return stub.err
	}
	return operation(ctx, stub.transaction)
}

func TestProfileUseCasesUseTrustedPrincipalAndTransaction(t *testing.T) {
	t.Parallel()

	principal := mustPrincipalID(t)
	want := profile.Profile{OwnerID: principal, ExperienceLevel: profile.ExperienceIntermediate, DisplayUnits: profile.UnitsMetric, Version: 1}
	repository := &profileRepositoryStub{createResult: want, getResult: want, updateResult: profile.Profile{OwnerID: principal, ExperienceLevel: profile.ExperienceAdvanced, DisplayUnits: profile.UnitsImperial, Version: 2}}
	transactions := &transactorStub{transaction: transactionStub{profiles: repository}}
	service, err := NewService(transactions, uuid.New)
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateProfile(context.Background(), principal, ProfileInput{ExperienceLevel: "intermediate", DisplayUnits: "metric"})
	if err != nil || created != want {
		t.Fatalf("CreateProfile() = (%+v, %v)", created, err)
	}
	if repository.created.OwnerID != principal || transactions.principal != principal {
		t.Fatal("CreateProfile() did not propagate the trusted principal")
	}

	got, err := service.GetProfile(context.Background(), principal)
	if err != nil || got != want || repository.getOwner != principal {
		t.Fatalf("GetProfile() = (%+v, %v)", got, err)
	}

	updated, err := service.UpdateProfile(context.Background(), principal, UpdateProfileInput{ExperienceLevel: "advanced", DisplayUnits: "imperial", ExpectedVersion: 1})
	if err != nil || updated.Version != 2 {
		t.Fatalf("UpdateProfile() = (%+v, %v)", updated, err)
	}
	if repository.updateCandidate.OwnerID != principal || repository.expectedVersion != 1 {
		t.Fatal("UpdateProfile() did not propagate owner and expected version")
	}
	if transactions.calls != 3 {
		t.Fatalf("transaction calls = %d, want 3", transactions.calls)
	}
}

func TestProfileValidationFailsBeforeTransaction(t *testing.T) {
	t.Parallel()

	transactions := &transactorStub{}
	service, err := NewService(transactions, uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	principal := mustPrincipalID(t)

	if _, err := service.CreateProfile(context.Background(), principal, ProfileInput{ExperienceLevel: "legend", DisplayUnits: "metric"}); err == nil {
		t.Fatal("CreateProfile() error = nil")
	}
	if _, err := service.UpdateProfile(context.Background(), principal, UpdateProfileInput{ExperienceLevel: "advanced", DisplayUnits: "metric", ExpectedVersion: 0}); err == nil {
		t.Fatal("UpdateProfile() error = nil")
	}
	if _, err := service.GetProfile(context.Background(), identity.PrincipalID{}); err == nil {
		t.Fatal("GetProfile() without principal error = nil")
	}
	if transactions.calls != 0 {
		t.Fatalf("transaction calls = %d, want 0", transactions.calls)
	}
}

func TestProfileUseCasesPropagateRepositoryAndTransactionErrors(t *testing.T) {
	t.Parallel()

	principal := mustPrincipalID(t)
	wantRepositoryError := errors.New("profile unavailable")
	repository := &profileRepositoryStub{getErr: wantRepositoryError}
	transactions := &transactorStub{transaction: transactionStub{profiles: repository}}
	service, err := NewService(transactions, uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetProfile(context.Background(), principal); !errors.Is(err, wantRepositoryError) {
		t.Fatalf("GetProfile() error = %v", err)
	}

	wantTransactionError := errors.New("rollback failed")
	transactions.err = wantTransactionError
	if _, err := service.GetProfile(context.Background(), principal); !errors.Is(err, wantTransactionError) {
		t.Fatalf("GetProfile() transaction error = %v", err)
	}
}

func TestNewServiceRequiresTransactor(t *testing.T) {
	t.Parallel()

	if _, err := NewService(nil, uuid.New); err == nil {
		t.Fatal("NewService(nil) error = nil")
	}
	if _, err := NewService(&transactorStub{}, nil); err == nil {
		t.Fatal("NewService() without ID generator error = nil")
	}
}

func mustPrincipalID(t *testing.T) identity.PrincipalID {
	t.Helper()
	id, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}
	return id
}
