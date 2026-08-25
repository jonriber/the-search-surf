package migrations_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jonriber/the-search-surf/backend/internal/platform/migrations"
)

func TestParseCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    migrations.Command
		wantErr string
	}{
		{name: "upgrade", args: []string{"up"}, want: migrations.CommandUp},
		{name: "status", args: []string{"status"}, want: migrations.CommandStatus},
		{name: "version", args: []string{"version"}, want: migrations.CommandVersion},
		{name: "missing", wantErr: "exactly one command"},
		{name: "too many", args: []string{"up", "status"}, wantErr: "exactly one command"},
		{name: "destructive command", args: []string{"down"}, wantErr: "unsupported migration command"},
		{name: "unknown", args: []string{"repair"}, wantErr: "unsupported migration command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := migrations.ParseCommand(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseCommand() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCommand() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunnerDispatchesCommandsAndWritesJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		command    migrations.Command
		wantCall   string
		wantOutput string
		provider   *fakeProvider
	}{
		{
			name:       "upgrade",
			command:    migrations.CommandUp,
			wantCall:   "up",
			wantOutput: `"applied_versions":[1,2]`,
			provider:   &fakeProvider{applied: []int64{1, 2}, version: 2},
		},
		{
			name:       "status",
			command:    migrations.CommandStatus,
			wantCall:   "status",
			wantOutput: `"state":"pending"`,
			provider: &fakeProvider{statuses: []migrations.Status{
				{Version: 1, Path: "00001_initial.sql", State: "applied"},
				{Version: 2, Path: "00002_next.sql", State: "pending"},
			}},
		},
		{
			name:       "version",
			command:    migrations.CommandVersion,
			wantCall:   "version",
			wantOutput: `"current_version":7`,
			provider:   &fakeProvider{version: 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			runner := migrations.NewRunner(tt.provider, &output)
			if err := runner.Run(context.Background(), tt.command); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if tt.provider.call != tt.wantCall {
				t.Fatalf("provider call = %q, want %q", tt.provider.call, tt.wantCall)
			}
			if !strings.Contains(output.String(), tt.wantOutput) {
				t.Fatalf("output = %q, want containing %q", output.String(), tt.wantOutput)
			}
		})
	}
}

func TestRunnerPropagatesProviderErrorsWithoutSuccessOutput(t *testing.T) {
	t.Parallel()

	providerError := errors.New("database unavailable")
	provider := &fakeProvider{err: providerError}
	var output bytes.Buffer
	runner := migrations.NewRunner(provider, &output)

	err := runner.Run(context.Background(), migrations.CommandUp)
	if !errors.Is(err, providerError) {
		t.Fatalf("Run() error = %v, want wrapping %v", err, providerError)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want empty", output.String())
	}
}

type fakeProvider struct {
	applied  []int64
	statuses []migrations.Status
	version  int64
	err      error
	call     string
}

func (p *fakeProvider) Up(context.Context) ([]int64, int64, error) {
	p.call = "up"
	return p.applied, p.version, p.err
}

func (p *fakeProvider) Status(context.Context) ([]migrations.Status, error) {
	p.call = "status"
	return p.statuses, p.err
}

func (p *fakeProvider) Version(context.Context) (int64, error) {
	p.call = "version"
	return p.version, p.err
}
