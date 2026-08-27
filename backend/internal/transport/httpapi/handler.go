// Package httpapi translates HTTP contracts into ownership-scoped user-data use cases.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jonriber/the-search-surf/backend/internal/application/userdata"
	"github.com/jonriber/the-search-surf/backend/internal/identity"
	"github.com/jonriber/the-search-surf/backend/internal/profile"
	"github.com/jonriber/the-search-surf/backend/internal/spots"
)

const (
	contentTypeJSON = "application/json; charset=utf-8"
	maxRequestBytes = 64 * 1024
)

// UserDataService declares only the application capabilities exposed over HTTP.
type UserDataService interface {
	CreateProfile(context.Context, identity.PrincipalID, userdata.ProfileInput) (profile.Profile, error)
	GetProfile(context.Context, identity.PrincipalID) (profile.Profile, error)
	UpdateProfile(context.Context, identity.PrincipalID, userdata.UpdateProfileInput) (profile.Profile, error)
	CreateSpot(context.Context, identity.PrincipalID, userdata.SpotInput) (spots.Spot, error)
	GetSpot(context.Context, identity.PrincipalID, uuid.UUID) (spots.Spot, error)
	ListSpots(context.Context, identity.PrincipalID) ([]spots.Spot, error)
	UpdateSpot(context.Context, identity.PrincipalID, uuid.UUID, userdata.UpdateSpotInput) (spots.Spot, error)
	DeleteSpot(context.Context, identity.PrincipalID, uuid.UUID, int64) error
	AddFavorite(context.Context, identity.PrincipalID, uuid.UUID, int) (spots.Favorite, error)
	ListFavorites(context.Context, identity.PrincipalID) ([]spots.Favorite, error)
	UpdateFavoritePosition(context.Context, identity.PrincipalID, uuid.UUID, int) (spots.Favorite, error)
	RemoveFavorite(context.Context, identity.PrincipalID, uuid.UUID) error
}

// PrincipalResolver establishes the acting principal from trusted server-side state.
type PrincipalResolver func(*http.Request) (identity.PrincipalID, error)

// FixedPrincipal resolves every request to the bootstrap principal used in single-user mode.
func FixedPrincipal(principal identity.PrincipalID) PrincipalResolver {
	return func(*http.Request) (identity.PrincipalID, error) {
		if principal.IsZero() {
			return identity.PrincipalID{}, errors.New("fixed principal is required")
		}
		return principal, nil
	}
}

// Options declares the HTTP adapter dependencies.
type Options struct {
	Service           UserDataService
	PrincipalResolver PrincipalResolver
	Logger            *slog.Logger
}

type handler struct {
	service           UserDataService
	principalResolver PrincipalResolver
	logger            *slog.Logger
}

// NewHandler creates the user-data HTTP handler without opening a listener.
func NewHandler(options Options) (http.Handler, error) {
	if options.Service == nil {
		return nil, errors.New("user-data service is required")
	}
	if options.PrincipalResolver == nil {
		return nil, errors.New("principal resolver is required")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	adapter := &handler{service: options.Service, principalResolver: options.PrincipalResolver, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /profile", adapter.createProfile)
	mux.HandleFunc("GET /profile", adapter.getProfile)
	mux.HandleFunc("PUT /profile", adapter.updateProfile)
	mux.HandleFunc("POST /spots", adapter.createSpot)
	mux.HandleFunc("GET /spots", adapter.listSpots)
	mux.HandleFunc("GET /spots/{spotID}", adapter.getSpot)
	mux.HandleFunc("PUT /spots/{spotID}", adapter.updateSpot)
	mux.HandleFunc("DELETE /spots/{spotID}", adapter.deleteSpot)
	mux.HandleFunc("POST /favorites", adapter.addFavorite)
	mux.HandleFunc("GET /favorites", adapter.listFavorites)
	mux.HandleFunc("PUT /favorites/{spotID}", adapter.updateFavoritePosition)
	mux.HandleFunc("DELETE /favorites/{spotID}", adapter.removeFavorite)

	return noStore(mux), nil
}

type profileRequest struct {
	ExperienceLevel string `json:"experienceLevel"`
	DisplayUnits    string `json:"displayUnits"`
}

type updateProfileRequest struct {
	ExperienceLevel string `json:"experienceLevel"`
	DisplayUnits    string `json:"displayUnits"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

type spotRequest struct {
	Name      string  `json:"name"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	TimeZone  string  `json:"timeZone"`
}

type updateSpotRequest struct {
	Name            string  `json:"name"`
	Longitude       float64 `json:"longitude"`
	Latitude        float64 `json:"latitude"`
	TimeZone        string  `json:"timeZone"`
	ExpectedVersion int64   `json:"expectedVersion"`
}

type favoriteRequest struct {
	SpotID       uuid.UUID `json:"spotId"`
	SortPosition int       `json:"sortPosition"`
}

type favoritePositionRequest struct {
	SortPosition int `json:"sortPosition"`
}

type profileResponse struct {
	ExperienceLevel string    `json:"experienceLevel"`
	DisplayUnits    string    `json:"displayUnits"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type spotResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Longitude float64   `json:"longitude"`
	Latitude  float64   `json:"latitude"`
	TimeZone  string    `json:"timeZone"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type favoriteResponse struct {
	SpotID       uuid.UUID `json:"spotId"`
	SortPosition int       `json:"sortPosition"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (adapter *handler) createProfile(w http.ResponseWriter, request *http.Request) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return
	}
	var input profileRequest
	if !decodeJSON(w, request, &input) {
		return
	}
	created, err := adapter.service.CreateProfile(request.Context(), principal, userdata.ProfileInput(input))
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProfileResponse(created))
}

func (adapter *handler) getProfile(w http.ResponseWriter, request *http.Request) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return
	}
	found, err := adapter.service.GetProfile(request.Context(), principal)
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, toProfileResponse(found))
}

func (adapter *handler) updateProfile(w http.ResponseWriter, request *http.Request) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return
	}
	var input updateProfileRequest
	if !decodeJSON(w, request, &input) {
		return
	}
	updated, err := adapter.service.UpdateProfile(request.Context(), principal, userdata.UpdateProfileInput(input))
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, toProfileResponse(updated))
}

func (adapter *handler) createSpot(w http.ResponseWriter, request *http.Request) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return
	}
	var input spotRequest
	if !decodeJSON(w, request, &input) {
		return
	}
	created, err := adapter.service.CreateSpot(request.Context(), principal, userdata.SpotInput(input))
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSpotResponse(created))
}

func (adapter *handler) listSpots(w http.ResponseWriter, request *http.Request) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return
	}
	found, err := adapter.service.ListSpots(request.Context(), principal)
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	items := make([]spotResponse, 0, len(found))
	for _, spot := range found {
		items = append(items, toSpotResponse(spot))
	}
	writeJSON(w, http.StatusOK, struct {
		Items []spotResponse `json:"items"`
	}{Items: items})
}

func (adapter *handler) getSpot(w http.ResponseWriter, request *http.Request) {
	principal, spotID, ok := adapter.resolvePrincipalAndSpot(w, request)
	if !ok {
		return
	}
	found, err := adapter.service.GetSpot(request.Context(), principal, spotID)
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, toSpotResponse(found))
}

func (adapter *handler) updateSpot(w http.ResponseWriter, request *http.Request) {
	principal, spotID, ok := adapter.resolvePrincipalAndSpot(w, request)
	if !ok {
		return
	}
	var input updateSpotRequest
	if !decodeJSON(w, request, &input) {
		return
	}
	updated, err := adapter.service.UpdateSpot(request.Context(), principal, spotID, userdata.UpdateSpotInput{
		SpotInput:       userdata.SpotInput{Name: input.Name, Longitude: input.Longitude, Latitude: input.Latitude, TimeZone: input.TimeZone},
		ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, toSpotResponse(updated))
}

func (adapter *handler) deleteSpot(w http.ResponseWriter, request *http.Request) {
	principal, spotID, ok := adapter.resolvePrincipalAndSpot(w, request)
	if !ok {
		return
	}
	expectedVersion, err := strconv.ParseInt(request.URL.Query().Get("expectedVersion"), 10, 64)
	if err != nil || expectedVersion <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "expectedVersion must be a positive integer")
		return
	}
	if err := adapter.service.DeleteSpot(request.Context(), principal, spotID, expectedVersion); err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (adapter *handler) addFavorite(w http.ResponseWriter, request *http.Request) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return
	}
	var input favoriteRequest
	if !decodeJSON(w, request, &input) {
		return
	}
	added, err := adapter.service.AddFavorite(request.Context(), principal, input.SpotID, input.SortPosition)
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusCreated, toFavoriteResponse(added))
}

func (adapter *handler) listFavorites(w http.ResponseWriter, request *http.Request) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return
	}
	found, err := adapter.service.ListFavorites(request.Context(), principal)
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	items := make([]favoriteResponse, 0, len(found))
	for _, favorite := range found {
		items = append(items, toFavoriteResponse(favorite))
	}
	writeJSON(w, http.StatusOK, struct {
		Items []favoriteResponse `json:"items"`
	}{Items: items})
}

func (adapter *handler) updateFavoritePosition(w http.ResponseWriter, request *http.Request) {
	principal, spotID, ok := adapter.resolvePrincipalAndSpot(w, request)
	if !ok {
		return
	}
	var input favoritePositionRequest
	if !decodeJSON(w, request, &input) {
		return
	}
	updated, err := adapter.service.UpdateFavoritePosition(request.Context(), principal, spotID, input.SortPosition)
	if err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	writeJSON(w, http.StatusOK, toFavoriteResponse(updated))
}

func (adapter *handler) removeFavorite(w http.ResponseWriter, request *http.Request) {
	principal, spotID, ok := adapter.resolvePrincipalAndSpot(w, request)
	if !ok {
		return
	}
	if err := adapter.service.RemoveFavorite(request.Context(), principal, spotID); err != nil {
		adapter.writeApplicationError(w, request, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (adapter *handler) resolvePrincipal(w http.ResponseWriter, request *http.Request) (identity.PrincipalID, bool) {
	principal, err := adapter.principalResolver(request)
	if err != nil || principal.IsZero() {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "a trusted principal is required")
		return identity.PrincipalID{}, false
	}
	return principal, true
}

func (adapter *handler) resolvePrincipalAndSpot(w http.ResponseWriter, request *http.Request) (identity.PrincipalID, uuid.UUID, bool) {
	principal, ok := adapter.resolvePrincipal(w, request)
	if !ok {
		return identity.PrincipalID{}, uuid.Nil, false
	}
	spotID, err := uuid.Parse(request.PathValue("spotID"))
	if err != nil || spotID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "spotId must be a non-nil UUID")
		return identity.PrincipalID{}, uuid.Nil, false
	}
	return principal, spotID, true
}

func (adapter *handler) writeApplicationError(w http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, userdata.ErrInvalidArgument):
		writeError(w, http.StatusBadRequest, "invalid_request", "request values are invalid")
	case errors.Is(err, userdata.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "the requested resource was not found")
	case errors.Is(err, userdata.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already_exists", "the resource already exists")
	case errors.Is(err, userdata.ErrConflict):
		writeError(w, http.StatusConflict, "version_conflict", "the resource changed; refresh and retry")
	default:
		adapter.logger.ErrorContext(request.Context(), "user-data HTTP operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "the request could not be completed")
	}
}

func decodeJSON(w http.ResponseWriter, request *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be one valid JSON object with known fields")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain exactly one JSON object")
		return false
	}
	return true
}

func toProfileResponse(value profile.Profile) profileResponse {
	return profileResponse{ExperienceLevel: string(value.ExperienceLevel), DisplayUnits: string(value.DisplayUnits), Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func toSpotResponse(value spots.Spot) spotResponse {
	return spotResponse{ID: value.ID, Name: value.Name, Longitude: value.Longitude, Latitude: value.Latitude, TimeZone: value.TimeZone, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func toFavoriteResponse(value spots.Favorite) favoriteResponse {
	return favoriteResponse{SpotID: value.SpotID, SortPosition: value.SortPosition, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Default().Error("encode user-data HTTP response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, request)
	})
}
