package forecasting

import (
	"fmt"
	"time"

	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

// MalformedResponseError carries checksum-only quarantine evidence while still
// supporting errors.Is(err, ErrMalformedResponse).
type MalformedResponseError struct {
	PointReference   string
	Component        forecast.Component
	FetchedAt        time.Time
	SHA256           string
	StorageReference string
	Reason           string
	Cause            error
}

func (failure *MalformedResponseError) Error() string {
	return fmt.Sprintf("%v: %s", ErrMalformedResponse, failure.Reason)
}

func (failure *MalformedResponseError) Unwrap() []error {
	errors := []error{ErrMalformedResponse}
	if failure.Cause != nil {
		errors = append(errors, failure.Cause)
	}
	return errors
}
