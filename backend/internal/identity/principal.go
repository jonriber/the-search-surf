// Package identity defines trusted internal identity handles without coupling
// them to an authentication protocol, HTTP, or persistence technology.
package identity

import (
	"errors"

	"github.com/google/uuid"
)

// PrincipalID is the stable internal owner identifier established by a trusted
// identity resolver. It is not itself a credential.
type PrincipalID uuid.UUID

// ParsePrincipalID parses a non-nil UUID into a principal identifier.
func ParsePrincipalID(value string) (PrincipalID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return PrincipalID{}, errors.New("principal ID must be a valid UUID")
	}
	if id == uuid.Nil {
		return PrincipalID{}, errors.New("principal ID must not be nil")
	}
	return PrincipalID(id), nil
}

// IsZero reports whether the identifier is missing.
func (id PrincipalID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

// String returns the canonical UUID representation.
func (id PrincipalID) String() string {
	return uuid.UUID(id).String()
}

// UUID returns the generic UUID representation for adapter boundaries.
func (id PrincipalID) UUID() uuid.UUID {
	return uuid.UUID(id)
}
