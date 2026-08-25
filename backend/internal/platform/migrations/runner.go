// Package migrations owns the forward-only database migration boundary.
package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Command is a safe operation exposed by the production migration entry point.
type Command string

const (
	// CommandUp applies every pending forward migration.
	CommandUp Command = "up"
	// CommandStatus reports the state of every embedded migration.
	CommandStatus Command = "status"
	// CommandVersion reports the highest applied migration version.
	CommandVersion Command = "version"
)

// State describes whether a migration is present in the database.
type State string

const (
	// StateApplied indicates that the migration has been recorded by the database.
	StateApplied State = "applied"
	// StatePending indicates that the migration has not been applied.
	StatePending State = "pending"
)

// Status is the stable application representation of one migration state.
type Status struct {
	Version int64  `json:"version"`
	Path    string `json:"path"`
	State   State  `json:"state"`
}

// Provider isolates command behavior from the selected migration library.
type Provider interface {
	Up(context.Context) (appliedVersions []int64, currentVersion int64, err error)
	Status(context.Context) ([]Status, error)
	Version(context.Context) (int64, error)
}

// Runner dispatches safe commands and writes one JSON result after success.
type Runner struct {
	provider Provider
	output   io.Writer
}

// NewRunner creates a migration command runner.
func NewRunner(provider Provider, output io.Writer) *Runner {
	return &Runner{provider: provider, output: output}
}

// ParseCommand validates the command-line operation allowlist.
func ParseCommand(args []string) (Command, error) {
	if len(args) != 1 {
		return "", errors.New("exactly one command is required: up, status, or version")
	}

	command := Command(args[0])
	switch command {
	case CommandUp, CommandStatus, CommandVersion:
		return command, nil
	default:
		return "", fmt.Errorf("unsupported migration command %q", args[0])
	}
}

// Run executes one allowlisted command.
func (r *Runner) Run(ctx context.Context, command Command) error {
	if r == nil || r.provider == nil {
		return errors.New("migration provider is required")
	}
	if r.output == nil {
		return errors.New("migration output is required")
	}

	switch command {
	case CommandUp:
		applied, current, err := r.provider.Up(ctx)
		if err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
		return writeJSON(r.output, struct {
			Command         Command `json:"command"`
			AppliedVersions []int64 `json:"applied_versions"`
			CurrentVersion  int64   `json:"current_version"`
		}{Command: command, AppliedVersions: applied, CurrentVersion: current})
	case CommandStatus:
		statuses, err := r.provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("get migration status: %w", err)
		}
		return writeJSON(r.output, struct {
			Command    Command  `json:"command"`
			Migrations []Status `json:"migrations"`
		}{Command: command, Migrations: statuses})
	case CommandVersion:
		version, err := r.provider.Version(ctx)
		if err != nil {
			return fmt.Errorf("get migration version: %w", err)
		}
		return writeJSON(r.output, struct {
			Command        Command `json:"command"`
			CurrentVersion int64   `json:"current_version"`
		}{Command: command, CurrentVersion: version})
	default:
		return fmt.Errorf("unsupported migration command %q", command)
	}
}

func writeJSON(output io.Writer, value any) error {
	if err := json.NewEncoder(output).Encode(value); err != nil {
		return fmt.Errorf("write migration result: %w", err)
	}
	return nil
}
