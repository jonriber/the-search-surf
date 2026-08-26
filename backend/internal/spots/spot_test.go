package spots

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

func TestNewSpotValidatesAndNormalizesInput(t *testing.T) {
	t.Parallel()

	owner, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}
	spotID := uuid.MustParse("4fda51a7-d38b-47b2-8c84-aaf455a73602")

	got, err := NewSpot(spotID, owner, "  Supertubos  ", -9.3645, 39.3394, "Europe/Lisbon")
	if err != nil {
		t.Fatalf("NewSpot() error = %v", err)
	}
	if got.Name != "Supertubos" || got.Longitude != -9.3645 || got.Latitude != 39.3394 || got.TimeZone != "Europe/Lisbon" || got.Version != 1 {
		t.Fatalf("NewSpot() = %+v", got)
	}
}

func TestNewSpotRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	owner, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}
	spotID := uuid.MustParse("4fda51a7-d38b-47b2-8c84-aaf455a73602")

	tests := []struct {
		name      string
		id        uuid.UUID
		owner     identity.PrincipalID
		spotName  string
		longitude float64
		latitude  float64
		timeZone  string
	}{
		{name: "nil id", owner: owner, spotName: "Spot", timeZone: "UTC"},
		{name: "nil owner", id: spotID, spotName: "Spot", timeZone: "UTC"},
		{name: "blank name", id: spotID, owner: owner, spotName: "  ", timeZone: "UTC"},
		{name: "long name", id: spotID, owner: owner, spotName: string(make([]byte, 121)), timeZone: "UTC"},
		{name: "invalid longitude", id: spotID, owner: owner, spotName: "Spot", longitude: 181, timeZone: "UTC"},
		{name: "invalid latitude", id: spotID, owner: owner, spotName: "Spot", latitude: -91, timeZone: "UTC"},
		{name: "unknown time zone", id: spotID, owner: owner, spotName: "Spot", timeZone: "Atlantic/Atlantis"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewSpot(tt.id, tt.owner, tt.spotName, tt.longitude, tt.latitude, tt.timeZone); err == nil {
				t.Fatal("NewSpot() error = nil")
			}
		})
	}
}

func TestNewFavoriteRejectsNegativePosition(t *testing.T) {
	t.Parallel()

	owner, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}
	spotID := uuid.MustParse("4fda51a7-d38b-47b2-8c84-aaf455a73602")

	if _, err := NewFavorite(owner, spotID, -1); err == nil {
		t.Fatal("NewFavorite() error = nil")
	}
}
