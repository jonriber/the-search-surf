package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/application/userdata"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/profile"
	"github.com/jonriber/the-search-surf/backend/internal/spots"
)

type userDataStub struct {
	principal identity.PrincipalID
	spotID    uuid.UUID
	profile   profile.Profile
	spot      spots.Spot
	favorite  spots.Favorite
	error     error
}

func (stub *userDataStub) CreateProfile(_ context.Context, principal identity.PrincipalID, _ userdata.ProfileInput) (profile.Profile, error) {
	stub.principal = principal
	return stub.profile, stub.error
}

func (stub *userDataStub) GetProfile(_ context.Context, principal identity.PrincipalID) (profile.Profile, error) {
	stub.principal = principal
	return stub.profile, stub.error
}

func (stub *userDataStub) UpdateProfile(_ context.Context, principal identity.PrincipalID, _ userdata.UpdateProfileInput) (profile.Profile, error) {
	stub.principal = principal
	return stub.profile, stub.error
}

func (stub *userDataStub) CreateSpot(_ context.Context, principal identity.PrincipalID, _ userdata.SpotInput) (spots.Spot, error) {
	stub.principal = principal
	return stub.spot, stub.error
}

func (stub *userDataStub) GetSpot(_ context.Context, principal identity.PrincipalID, spotID uuid.UUID) (spots.Spot, error) {
	stub.principal, stub.spotID = principal, spotID
	return stub.spot, stub.error
}

func (stub *userDataStub) ListSpots(_ context.Context, principal identity.PrincipalID) ([]spots.Spot, error) {
	stub.principal = principal
	return []spots.Spot{stub.spot}, stub.error
}

func (stub *userDataStub) UpdateSpot(_ context.Context, principal identity.PrincipalID, spotID uuid.UUID, _ userdata.UpdateSpotInput) (spots.Spot, error) {
	stub.principal, stub.spotID = principal, spotID
	return stub.spot, stub.error
}

func (stub *userDataStub) DeleteSpot(_ context.Context, principal identity.PrincipalID, spotID uuid.UUID, _ int64) error {
	stub.principal, stub.spotID = principal, spotID
	return stub.error
}

func (stub *userDataStub) AddFavorite(_ context.Context, principal identity.PrincipalID, spotID uuid.UUID, _ int) (spots.Favorite, error) {
	stub.principal, stub.spotID = principal, spotID
	return stub.favorite, stub.error
}

func (stub *userDataStub) ListFavorites(_ context.Context, principal identity.PrincipalID) ([]spots.Favorite, error) {
	stub.principal = principal
	return []spots.Favorite{stub.favorite}, stub.error
}

func (stub *userDataStub) UpdateFavoritePosition(_ context.Context, principal identity.PrincipalID, spotID uuid.UUID, _ int) (spots.Favorite, error) {
	stub.principal, stub.spotID = principal, spotID
	return stub.favorite, stub.error
}

func (stub *userDataStub) RemoveFavorite(_ context.Context, principal identity.PrincipalID, spotID uuid.UUID) error {
	stub.principal, stub.spotID = principal, spotID
	return stub.error
}

func TestUserDataHTTPContracts(t *testing.T) {
	t.Parallel()

	principal := mustPrincipal(t)
	spotID := uuid.MustParse("4fda51a7-d38b-47b2-8c84-aaf455a73602")
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.UTC)
	stub := &userDataStub{
		profile:  profile.Profile{OwnerID: principal, ExperienceLevel: profile.ExperienceIntermediate, DisplayUnits: profile.UnitsMetric, Version: 1, CreatedAt: now, UpdatedAt: now},
		spot:     spots.Spot{ID: spotID, OwnerID: principal, Name: "Supertubos", Longitude: -9.3645, Latitude: 39.3394, TimeZone: "Europe/Lisbon", Version: 1, CreatedAt: now, UpdatedAt: now},
		favorite: spots.Favorite{OwnerID: principal, SpotID: spotID, SortPosition: 2, CreatedAt: now, UpdatedAt: now},
	}
	handler, err := NewHandler(Options{Service: stub, PrincipalResolver: FixedPrincipal(principal)})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		status       int
		bodyContains string
	}{
		{name: "create profile", method: http.MethodPost, path: "/profile", body: `{"experienceLevel":"intermediate","displayUnits":"metric"}`, status: http.StatusCreated, bodyContains: `"experienceLevel":"intermediate"`},
		{name: "get profile", method: http.MethodGet, path: "/profile", status: http.StatusOK, bodyContains: `"version":1`},
		{name: "update profile", method: http.MethodPut, path: "/profile", body: `{"experienceLevel":"intermediate","displayUnits":"metric","expectedVersion":1}`, status: http.StatusOK, bodyContains: `"displayUnits":"metric"`},
		{name: "create spot", method: http.MethodPost, path: "/spots", body: `{"name":"Supertubos","longitude":-9.3645,"latitude":39.3394,"timeZone":"Europe/Lisbon"}`, status: http.StatusCreated, bodyContains: `"id":"4fda51a7-d38b-47b2-8c84-aaf455a73602"`},
		{name: "list spots", method: http.MethodGet, path: "/spots", status: http.StatusOK, bodyContains: `"items":[{"id":"4fda51a7-d38b-47b2-8c84-aaf455a73602"`},
		{name: "get spot", method: http.MethodGet, path: "/spots/" + spotID.String(), status: http.StatusOK, bodyContains: `"longitude":-9.3645`},
		{name: "update spot", method: http.MethodPut, path: "/spots/" + spotID.String(), body: `{"name":"Supertubos","longitude":-9.3645,"latitude":39.3394,"timeZone":"Europe/Lisbon","expectedVersion":1}`, status: http.StatusOK, bodyContains: `"timeZone":"Europe/Lisbon"`},
		{name: "delete spot", method: http.MethodDelete, path: "/spots/" + spotID.String() + "?expectedVersion=1", status: http.StatusNoContent},
		{name: "add favorite", method: http.MethodPost, path: "/favorites", body: `{"spotId":"4fda51a7-d38b-47b2-8c84-aaf455a73602","sortPosition":2}`, status: http.StatusCreated, bodyContains: `"sortPosition":2`},
		{name: "list favorites", method: http.MethodGet, path: "/favorites", status: http.StatusOK, bodyContains: `"items":[{"spotId":"4fda51a7-d38b-47b2-8c84-aaf455a73602"`},
		{name: "reorder favorite", method: http.MethodPut, path: "/favorites/" + spotID.String(), body: `{"sortPosition":2}`, status: http.StatusOK, bodyContains: `"spotId":"4fda51a7-d38b-47b2-8c84-aaf455a73602"`},
		{name: "remove favorite", method: http.MethodDelete, path: "/favorites/" + spotID.String(), status: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *bytes.Reader
			if tt.body == "" {
				body = bytes.NewReader(nil)
			} else {
				body = bytes.NewReader([]byte(tt.body))
			}
			request := httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, body)
			if tt.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tt.status, response.Body.String())
			}
			if tt.bodyContains != "" && !strings.Contains(response.Body.String(), tt.bodyContains) {
				t.Fatalf("body = %q, want substring %q", response.Body.String(), tt.bodyContains)
			}
			if stub.principal != principal {
				t.Fatalf("principal = %s, want trusted principal %s", stub.principal, principal)
			}
			if strings.Contains(response.Body.String(), "ownerId") {
				t.Fatalf("response leaks owner identifier: %s", response.Body.String())
			}
		})
	}
}

func TestHTTPBoundaryRejectsCallerOwnedIdentityAndMalformedInput(t *testing.T) {
	t.Parallel()

	principal := mustPrincipal(t)
	stub := &userDataStub{}
	handler, err := NewHandler(Options{Service: stub, PrincipalResolver: FixedPrincipal(principal)})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		status      int
	}{
		{name: "owner field", method: http.MethodPost, path: "/profile", contentType: "application/json", body: `{"ownerId":"1f5cb40c-194c-438f-a606-e12588989d0f","experienceLevel":"beginner","displayUnits":"metric"}`, status: http.StatusBadRequest},
		{name: "unknown field", method: http.MethodPost, path: "/spots", contentType: "application/json", body: `{"name":"spot","timeZone":"UTC","surprise":true}`, status: http.StatusBadRequest},
		{name: "wrong media type", method: http.MethodPost, path: "/profile", contentType: "text/plain", body: `{}`, status: http.StatusUnsupportedMediaType},
		{name: "oversized body", method: http.MethodPost, path: "/profile", contentType: "application/json", body: `{"experienceLevel":"` + strings.Repeat("x", maxRequestBytes) + `"}`, status: http.StatusRequestEntityTooLarge},
		{name: "invalid path UUID", method: http.MethodGet, path: "/spots/not-a-uuid", status: http.StatusBadRequest},
		{name: "invalid delete version", method: http.MethodDelete, path: "/spots/4fda51a7-d38b-47b2-8c84-aaf455a73602?expectedVersion=zero", status: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, strings.NewReader(tt.body))
			if tt.contentType != "" {
				request.Header.Set("Content-Type", tt.contentType)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tt.status, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestApplicationErrorsHaveStableHTTPTranslation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid", err: userdata.ErrInvalidArgument, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "not found", err: userdata.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
		{name: "already exists", err: userdata.ErrAlreadyExists, status: http.StatusConflict, code: "already_exists"},
		{name: "version conflict", err: userdata.ErrConflict, status: http.StatusConflict, code: "version_conflict"},
		{name: "internal", err: errors.New("password=must-not-leak"), status: http.StatusInternalServerError, code: "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &userDataStub{error: tt.err}
			handler, err := NewHandler(Options{Service: stub, PrincipalResolver: FixedPrincipal(mustPrincipal(t))})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/profile", nil))
			if response.Code != tt.status || !strings.Contains(response.Body.String(), `"code":"`+tt.code+`"`) {
				t.Fatalf("response = (%d, %s)", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "must-not-leak") {
				t.Fatalf("response leaks internal error: %s", response.Body.String())
			}
		})
	}
}

func TestNewHandlerRequiresDependencies(t *testing.T) {
	t.Parallel()

	if _, err := NewHandler(Options{}); err == nil {
		t.Fatal("NewHandler() error = nil")
	}
	if _, err := NewHandler(Options{Service: &userDataStub{}}); err == nil {
		t.Fatal("NewHandler() without principal resolver error = nil")
	}
}

func mustPrincipal(t *testing.T) identity.PrincipalID {
	t.Helper()
	principal, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}
	return principal
}
