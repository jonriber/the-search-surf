// Package healthcheck provides the self-probe used by the minimal runtime image.
package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// HTTPClient is the smallest client contract needed by Check.
type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

// Check calls the API readiness endpoint through its loopback interface.
func Check(ctx context.Context, listenAddress string, client HTTPClient) error {
	if client == nil {
		return fmt.Errorf("HTTP client is required")
	}

	_, port, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return fmt.Errorf("parse listen address: %w", err)
	}

	endpoint := "http://" + net.JoinHostPort("127.0.0.1", port) + "/health/ready"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request readiness endpoint: %w", err)
	}
	statusCode := response.StatusCode
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close readiness response: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("readiness endpoint returned status %d", statusCode)
	}

	return nil
}
