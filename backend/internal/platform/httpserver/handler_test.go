package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthAndVersionContracts(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{Version: Version{Release: "1.2.3", Commit: "abc123"}})

	tests := []struct {
		name   string
		path   string
		status int
		body   string
	}{
		{name: "liveness", path: "/health/live", status: http.StatusOK, body: `{"status":"ok"}`},
		{name: "readiness", path: "/health/ready", status: http.StatusOK, body: `{"status":"ready"}`},
		{name: "version", path: "/version", status: http.StatusOK, body: `{"version":"1.2.3","commit":"abc123"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d", response.Code, tt.status)
			}
			if strings.TrimSpace(response.Body.String()) != tt.body {
				t.Fatalf("body = %q, want %q", strings.TrimSpace(response.Body.String()), tt.body)
			}
			if response.Header().Get("Content-Type") != contentTypeJSON {
				t.Fatalf("Content-Type = %q, want %q", response.Header().Get("Content-Type"), contentTypeJSON)
			}
			if response.Header().Get(requestIDHeader) == "" {
				t.Fatal("X-Request-ID is empty")
			}
		})
	}
}

func TestReadinessFailure(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{
		ReadinessCheck: func(context.Context) error { return errors.New("dependency unavailable") },
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/ready", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if strings.TrimSpace(response.Body.String()) != `{"status":"unavailable"}` {
		t.Fatalf("body = %q, want unavailable contract", strings.TrimSpace(response.Body.String()))
	}
}

func TestRequestIDHandling(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{})

	t.Run("preserves valid caller identifier", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/live", nil)
		request.Header.Set(requestIDHeader, "caller-123")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if got := response.Header().Get(requestIDHeader); got != "caller-123" {
			t.Fatalf("X-Request-ID = %q, want %q", got, "caller-123")
		}
	})

	t.Run("replaces unsafe caller identifier", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/live", nil)
		request.Header.Set(requestIDHeader, "unsafe identifier")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if got := response.Header().Get(requestIDHeader); got == "" || got == "unsafe identifier" {
			t.Fatalf("X-Request-ID = %q, want generated identifier", got)
		}
	})
}

func TestMethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/health/live", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestApplicationHandlerComposition(t *testing.T) {
	t.Parallel()

	application := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := NewHandler(HandlerOptions{ApplicationHandler: application})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/profile", nil))

	if response.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTeapot)
	}
}
