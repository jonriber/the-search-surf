package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRunRequiresExplicitConfigurationBeforeConnecting(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), func(string) (string, bool) { return "", false }, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "BOOTSTRAP_DATABASE_URL is required") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunRejectsInvalidPrincipalWithoutLeakingDatabaseURL(t *testing.T) {
	t.Parallel()

	sensitive := strings.Join([]string{"sentinel", "secret"}, "-")
	lookup := func(key string) (string, bool) {
		values := map[string]string{
			"BOOTSTRAP_DATABASE_URL": "postgres://user:" + sensitive + "@database/the_search",
			"BOOTSTRAP_PRINCIPAL_ID": "not-a-uuid",
		}
		value, ok := values[key]
		return value, ok
	}
	err := run(context.Background(), lookup, io.Discard)
	if err == nil || strings.Contains(err.Error(), sensitive) {
		t.Fatalf("run() error = %v", err)
	}
}
