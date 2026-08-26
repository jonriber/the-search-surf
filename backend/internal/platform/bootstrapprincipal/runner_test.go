package bootstrapprincipal

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jonriber/the-search-surf/backend/internal/identity"
)

type provisionerStub struct {
	created bool
	err     error
	gotID   identity.PrincipalID
}

func (stub *provisionerStub) Ensure(_ context.Context, id identity.PrincipalID) (bool, error) {
	stub.gotID = id
	return stub.created, stub.err
}

func TestRunnerReportsCreatedAndExistingStates(t *testing.T) {
	t.Parallel()

	id, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name    string
		created bool
		state   string
	}{
		{name: "created", created: true, state: "created"},
		{name: "existing", created: false, state: "existing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := &provisionerStub{created: tt.created}
			var output bytes.Buffer
			runner := Runner{Provisioner: provider, Output: &output}

			if err := runner.Run(context.Background(), id); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if provider.gotID != id {
				t.Fatalf("provisioned ID = %s", provider.gotID.String())
			}
			if got := strings.TrimSpace(output.String()); got != `{"status":"`+tt.state+`"}` {
				t.Fatalf("output = %q", got)
			}
		})
	}
}

func TestRunnerRejectsMissingDependenciesAndPropagatesProvisioningFailure(t *testing.T) {
	t.Parallel()

	id, err := identity.ParsePrincipalID("2f404f62-3d6f-4e5f-a2e8-1be44b08f05c")
	if err != nil {
		t.Fatal(err)
	}

	if err := (Runner{}).Run(context.Background(), id); err == nil {
		t.Fatal("Run() without provisioner error = nil")
	}
	want := errors.New("disabled principal")
	got := (Runner{Provisioner: &provisionerStub{err: want}, Output: &bytes.Buffer{}}).Run(context.Background(), id)
	if !errors.Is(got, want) {
		t.Fatalf("Run() error = %v, want wrapped provisioning error", got)
	}
}
