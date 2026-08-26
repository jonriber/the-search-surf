package bootstrapprincipal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

// Provisioner ensures that one configured principal exists and remains enabled.
type Provisioner interface {
	Ensure(context.Context, identity.PrincipalID) (created bool, err error)
}

// Runner executes and reports the one-shot provisioning operation.
type Runner struct {
	Provisioner Provisioner
	Output      io.Writer
}

// Run provisions the principal and writes a stable machine-readable result.
func (runner Runner) Run(ctx context.Context, principalID identity.PrincipalID) error {
	if runner.Provisioner == nil {
		return errors.New("bootstrap principal provisioner is required")
	}
	if runner.Output == nil {
		return errors.New("bootstrap principal output is required")
	}

	created, err := runner.Provisioner.Ensure(ctx, principalID)
	if err != nil {
		return fmt.Errorf("ensure bootstrap principal: %w", err)
	}
	status := "existing"
	if created {
		status = "created"
	}
	if err := json.NewEncoder(runner.Output).Encode(map[string]string{"status": status}); err != nil {
		return fmt.Errorf("encode bootstrap result: %w", err)
	}
	return nil
}
