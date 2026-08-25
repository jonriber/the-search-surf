// Command migrate applies and inspects The Search database migrations.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonriber/the-search-surf/backend/internal/platform/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, lookup migrations.LookupEnv, output io.Writer) error {
	command, err := migrations.ParseCommand(args)
	if err != nil {
		return err
	}
	config, err := migrations.ConfigFromEnvironment(lookup)
	if err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	database, err := sql.Open("pgx", config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	if err := database.PingContext(operationContext); err != nil {
		return errors.Join(fmt.Errorf("ping migration database: %w", err), database.Close())
	}

	provider, err := migrations.NewGooseProvider(database)
	if err != nil {
		return errors.Join(err, database.Close())
	}
	runErr := migrations.NewRunner(provider, output).Run(operationContext, command)
	return errors.Join(runErr, database.Close())
}
