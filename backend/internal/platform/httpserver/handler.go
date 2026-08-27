// Package httpserver owns the API process HTTP transport and middleware.
package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

const contentTypeJSON = "application/json; charset=utf-8"

// ReadinessCheck verifies whether the process can safely receive traffic.
type ReadinessCheck func(context.Context) error

// Version identifies the running release artifact.
type Version struct {
	Release string `json:"version"`
	Commit  string `json:"commit"`
}

// HandlerOptions declares the transport dependencies explicitly.
type HandlerOptions struct {
	Logger             *slog.Logger
	Version            Version
	ReadinessCheck     ReadinessCheck
	ApplicationHandler http.Handler
}

// NewHandler creates the complete API HTTP handler.
func NewHandler(options HandlerOptions) http.Handler {
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	readiness := options.ReadinessCheck
	if readiness == nil {
		readiness = func(context.Context) error { return nil }
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := readiness(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, options.Version)
	})
	if options.ApplicationHandler != nil {
		mux.Handle("/", options.ApplicationHandler)
	}

	return requestLogging(logger, requestID(securityHeaders(mux)))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Default().Error("encode HTTP response", "error", err)
	}
}
