// Command bootstrap-principal provisions the stable bootstrap identity before
// The Search API starts.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonriber/the-search-surf/backend/internal/platform/bootstrapprincipal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.LookupEnv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap principal failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, lookup bootstrapprincipal.LookupEnv, output io.Writer) error {
	config, err := bootstrapprincipal.ConfigFromEnvironment(lookup)
	if err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	connection, err := pgx.Connect(operationContext, config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect bootstrap database: %w", err)
	}

	provisioner := bootstrapprincipal.NewPostgresProvisioner(connection)
	runErr := (bootstrapprincipal.Runner{Provisioner: provisioner, Output: output}).Run(operationContext, config.PrincipalID)
	closeContext, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCancel()
	return errors.Join(runErr, connection.Close(closeContext))
}
