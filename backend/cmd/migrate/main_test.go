package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRunRejectsUnsafeCommandBeforeLoadingCredentials(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), []string{"down"}, nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unsupported migration command") {
		t.Fatalf("run() error = %v, want unsafe-command rejection", err)
	}
}

func TestRunRequiresMigrationConfiguration(t *testing.T) {
	t.Parallel()

	lookup := func(string) (string, bool) { return "", false }
	err := run(context.Background(), []string{"up"}, lookup, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "MIGRATION_DATABASE_URL is required") {
		t.Fatalf("run() error = %v, want missing configuration", err)
	}
}
