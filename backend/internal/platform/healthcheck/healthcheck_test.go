package healthcheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantError  bool
	}{
		{name: "ready", statusCode: http.StatusOK},
		{name: "unavailable", statusCode: http.StatusServiceUnavailable, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != "http://127.0.0.1:8080/health/ready" {
					t.Fatalf("URL = %q, want loopback readiness URL", request.URL.String())
				}
				return &http.Response{
					StatusCode: tt.statusCode,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			})

			err := Check(context.Background(), "0.0.0.0:8080", client)
			if (err != nil) != tt.wantError {
				t.Fatalf("Check() error = %v, wantError %t", err, tt.wantError)
			}
		})
	}
}

func TestCheckRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()

	if err := Check(context.Background(), ":8080", nil); err == nil {
		t.Fatal("Check() error = nil, want missing client error")
	}
	if err := Check(context.Background(), "invalid", http.DefaultClient); err == nil {
		t.Fatal("Check() error = nil, want invalid address error")
	}
}

func TestCheckReturnsClientError(t *testing.T) {
	t.Parallel()

	client := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network failure")
	})
	if err := Check(context.Background(), ":8080", client); err == nil {
		t.Fatal("Check() error = nil, want client error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}
