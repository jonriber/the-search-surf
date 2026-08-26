package identity

import "testing"

func TestParsePrincipalID(t *testing.T) {
	t.Parallel()

	id, err := ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatalf("ParsePrincipalID() error = %v", err)
	}
	if got := id.String(); got != "2f404f62-3d6f-4e5f-a2e8-1be44b08f05c" {
		t.Fatalf("PrincipalID.String() = %q", got)
	}
}

func TestParsePrincipalIDRejectsInvalidOrNilUUID(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		if _, err := ParsePrincipalID(value); err == nil {
			t.Fatalf("ParsePrincipalID(%q) error = nil", value)
		}
	}
}
