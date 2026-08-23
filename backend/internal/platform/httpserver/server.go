package httpserver

import (
	"net/http"

	"github.com/jonriber/the-search-surf/backend/internal/platform/config"
)

// New creates a configured HTTP server without starting network listeners.
func New(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}
