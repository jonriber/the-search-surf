package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx for the project-owned migration binary.
	"github.com/pressly/goose/v3"
)

const migrationTable = "the_search_schema_migrations"

//go:embed sql/*.sql
var embeddedSQL embed.FS

// NewGooseProvider creates the selected migration-library adapter over embedded SQL files.
func NewGooseProvider(db *sql.DB) (Provider, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}

	migrationFS, err := fs.Sub(embeddedSQL, "sql")
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrationFS,
		goose.WithTableName(migrationTable),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return nil, fmt.Errorf("create goose provider: %w", err)
	}

	return &gooseProvider{provider: provider}, nil
}

type gooseAPI interface {
	Up(context.Context) ([]*goose.MigrationResult, error)
	Status(context.Context) ([]*goose.MigrationStatus, error)
	GetDBVersion(context.Context) (int64, error)
}

type gooseProvider struct {
	provider gooseAPI
}

func (p *gooseProvider) Up(ctx context.Context) ([]int64, int64, error) {
	results, err := p.provider.Up(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("run goose upgrade: %w", err)
	}

	versions := make([]int64, 0, len(results))
	for _, result := range results {
		if result == nil || result.Source == nil {
			return nil, 0, fmt.Errorf("goose returned a migration result without a source")
		}
		versions = append(versions, result.Source.Version)
	}
	currentVersion, err := p.provider.GetDBVersion(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("get goose database version: %w", err)
	}

	return versions, currentVersion, nil
}

func (p *gooseProvider) Status(ctx context.Context) ([]Status, error) {
	gooseStatuses, err := p.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("get goose migration status: %w", err)
	}

	statuses := make([]Status, 0, len(gooseStatuses))
	for _, status := range gooseStatuses {
		if status == nil || status.Source == nil {
			return nil, fmt.Errorf("goose returned a migration status without a source")
		}
		statuses = append(statuses, Status{
			Version: status.Source.Version,
			Path:    status.Source.Path,
			State:   State(status.State),
		})
	}
	return statuses, nil
}

func (p *gooseProvider) Version(ctx context.Context) (int64, error) {
	version, err := p.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("get goose database version: %w", err)
	}
	return version, nil
}
