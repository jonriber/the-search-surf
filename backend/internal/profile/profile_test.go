package profile

import (
	"testing"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

func TestNewValidatesAndBuildsProfile(t *testing.T) {
	t.Parallel()

	owner, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}

	got, err := New(owner, "intermediate", "metric")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got.OwnerID != owner || got.ExperienceLevel != ExperienceIntermediate || got.DisplayUnits != UnitsMetric || got.Version != 1 {
		t.Fatalf("New() = %+v", got)
	}
}

func TestRestoreValidatesPersistedProfileState(t *testing.T) {
	t.Parallel()

	owner, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, time.August, 26, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	got, err := Restore(owner, "advanced", "imperial", 3, createdAt, updatedAt)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got.Version != 3 || got.CreatedAt != createdAt || got.UpdatedAt != updatedAt {
		t.Fatalf("Restore() = %+v", got)
	}

	if _, err := Restore(owner, "advanced", "imperial", 0, createdAt, updatedAt); err == nil {
		t.Fatal("Restore() with zero version error = nil")
	}
	if _, err := Restore(owner, "advanced", "imperial", 1, updatedAt, createdAt); err == nil {
		t.Fatal("Restore() with reversed timestamps error = nil")
	}
}

func TestNewRejectsInvalidProfileValues(t *testing.T) {
	t.Parallel()

	owner, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		owner      identity.PrincipalID
		experience string
		units      string
	}{
		{name: "missing owner", experience: "beginner", units: "metric"},
		{name: "unknown experience", owner: owner, experience: "legend", units: "metric"},
		{name: "unknown units", owner: owner, experience: "beginner", units: "nautical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(tt.owner, tt.experience, tt.units); err == nil {
				t.Fatal("New() error = nil")
			}
		})
	}
}
