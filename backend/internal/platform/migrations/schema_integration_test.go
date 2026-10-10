//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/jonriber/the-search-surf/backend/internal/platform/migrations"
)

const (
	principalA = "11111111-1111-4111-8111-111111111111"
	principalB = "22222222-2222-4222-8222-222222222222"
	spotA      = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	spotB      = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

func TestInitialSchemaContract(t *testing.T) {
	ctx := context.Background()
	migrationDB := openRequiredDatabase(t, "TEST_MIGRATION_DATABASE_URL")
	applicationDB := openRequiredDatabase(t, "TEST_APPLICATION_DATABASE_URL")
	ingesterDB := openRequiredDatabase(t, "TEST_INGESTER_DATABASE_URL")

	provider, err := migrations.NewGooseProvider(migrationDB)
	if err != nil {
		t.Fatalf("NewGooseProvider() error = %v", err)
	}

	t.Run("applies every migration once from an empty database", func(t *testing.T) {
		applied, version, err := provider.Up(ctx)
		if err != nil {
			t.Fatalf("Up() error = %v", err)
		}
		if !slices.Equal(applied, []int64{1, 2, 3}) {
			t.Fatalf("applied versions = %v, want [1 2 3]", applied)
		}
		if version != 3 {
			t.Fatalf("version = %d, want 3", version)
		}

		applied, version, err = provider.Up(ctx)
		if err != nil {
			t.Fatalf("second Up() error = %v", err)
		}
		if len(applied) != 0 || version != 3 {
			t.Fatalf("second Up() = (%v, %d), want ([], 3)", applied, version)
		}
	})

	t.Run("reports embedded migration status and version", func(t *testing.T) {
		statuses, err := provider.Status(ctx)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if len(statuses) != 3 || statuses[0].Version != 1 || statuses[1].Version != 2 || statuses[2].Version != 3 ||
			statuses[0].State != migrations.StateApplied || statuses[1].State != migrations.StateApplied || statuses[2].State != migrations.StateApplied {
			t.Fatalf("statuses = %#v, want applied versions 1, 2, and 3", statuses)
		}
		version, err := provider.Version(ctx)
		if err != nil || version != 3 {
			t.Fatalf("Version() = (%d, %v), want (3, nil)", version, err)
		}
	})

	t.Run("requires PostGIS and installs the spatial index", func(t *testing.T) {
		var extensionVersion string
		if err := migrationDB.QueryRowContext(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'postgis'").Scan(&extensionVersion); err != nil {
			t.Fatalf("query PostGIS extension: %v", err)
		}
		if extensionVersion == "" {
			t.Fatal("PostGIS extension version is empty")
		}

		var indexExists bool
		if err := migrationDB.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'public' AND indexname = 'surf_spots_position_gist_idx'
			)`).Scan(&indexExists); err != nil {
			t.Fatalf("query spatial index: %v", err)
		}
		if !indexExists {
			t.Fatal("spatial index does not exist")
		}
		if err := migrationDB.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'public' AND indexname = 'forecast_points_position_gist_idx'
			)`).Scan(&indexExists); err != nil {
			t.Fatalf("query forecast spatial index: %v", err)
		}
		if !indexExists {
			t.Fatal("forecast spatial index does not exist")
		}
	})

	seedPrincipals(t, ctx, migrationDB)

	t.Run("gives the runtime role no principal-table access", func(t *testing.T) {
		var canSelect, canInsert bool
		if err := migrationDB.QueryRowContext(ctx, `
			SELECT
				has_table_privilege('the_search_app', 'public.principals', 'SELECT'),
				has_table_privilege('the_search_app', 'public.principals', 'INSERT')
		`).Scan(&canSelect, &canInsert); err != nil {
			t.Fatalf("query runtime privileges: %v", err)
		}
		if canSelect || canInsert {
			t.Fatalf("runtime principal privileges = select:%t insert:%t, want false/false", canSelect, canInsert)
		}
	})

	t.Run("keeps migration and runtime roles least privileged", func(t *testing.T) {
		rows, err := migrationDB.QueryContext(ctx, `
			SELECT rolname, rolsuper, rolcreatedb, rolcreaterole, rolbypassrls
			FROM pg_roles
			WHERE rolname IN ('the_search_migrator', 'the_search_app', 'the_search_ingester')
			ORDER BY rolname
		`)
		if err != nil {
			t.Fatalf("query role privileges: %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var role string
			var superuser, createDatabase, createRole, bypassRLS bool
			if err := rows.Scan(&role, &superuser, &createDatabase, &createRole, &bypassRLS); err != nil {
				t.Fatalf("scan role privileges: %v", err)
			}
			if superuser || createDatabase || createRole || bypassRLS {
				t.Errorf(
					"%s privileges = superuser:%t createdb:%t createrole:%t bypassrls:%t, want all false",
					role,
					superuser,
					createDatabase,
					createRole,
					bypassRLS,
				)
			}
			count++
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate role privileges: %v", err)
		}
		if count != 3 {
			t.Fatalf("role count = %d, want 3", count)
		}

		var migratorCanCreate, applicationCanCreate, ingesterCanCreate bool
		if err := migrationDB.QueryRowContext(ctx, `
			SELECT
				has_schema_privilege('the_search_migrator', 'public', 'CREATE'),
				has_schema_privilege('the_search_app', 'public', 'CREATE'),
				has_schema_privilege('the_search_ingester', 'public', 'CREATE')
		`).Scan(&migratorCanCreate, &applicationCanCreate, &ingesterCanCreate); err != nil {
			t.Fatalf("query schema privileges: %v", err)
		}
		if !migratorCanCreate || applicationCanCreate || ingesterCanCreate {
			t.Fatalf(
				"schema create privileges = migrator:%t application:%t ingester:%t, want true/false/false",
				migratorCanCreate,
				applicationCanCreate,
				ingesterCanCreate,
			)
		}
	})

	t.Run("isolates ingestion from private and operational data", func(t *testing.T) {
		var canReadPoints, canWriteBatches, canWriteQuota bool
		if err := migrationDB.QueryRowContext(ctx, `
			SELECT
				has_table_privilege('the_search_ingester', 'public.forecast_points', 'SELECT'),
				has_table_privilege('the_search_ingester', 'public.forecast_batches', 'INSERT'),
				has_table_privilege('the_search_ingester', 'public.provider_quota_usage', 'UPDATE')
		`).Scan(&canReadPoints, &canWriteBatches, &canWriteQuota); err != nil {
			t.Fatalf("query ingestion privileges: %v", err)
		}
		if !canReadPoints || !canWriteBatches || !canWriteQuota {
			t.Fatalf(
				"ingestion privileges = points:%t batches:%t quota:%t, want true/true/true",
				canReadPoints,
				canWriteBatches,
				canWriteQuota,
			)
		}

		var canReadPrincipals, canReadSpots, appCanReadPayloads bool
		if err := migrationDB.QueryRowContext(ctx, `
			SELECT
				has_table_privilege('the_search_ingester', 'public.principals', 'SELECT'),
				has_table_privilege('the_search_ingester', 'public.surf_spots', 'SELECT'),
				has_table_privilege('the_search_app', 'public.forecast_payloads', 'SELECT')
		`).Scan(&canReadPrincipals, &canReadSpots, &appCanReadPayloads); err != nil {
			t.Fatalf("query denied ingestion privileges: %v", err)
		}
		if canReadPrincipals || canReadSpots || appCanReadPayloads {
			t.Fatalf(
				"denied privileges = principals:%t spots:%t application-payloads:%t, want false/false/false",
				canReadPrincipals,
				canReadSpots,
				appCanReadPayloads,
			)
		}

		if err := ingesterDB.QueryRowContext(ctx, "SELECT count(*) FROM principals").Scan(new(int)); err == nil {
			t.Fatal("ingester selected private principals, want permission denial")
		}
	})

	t.Run("enables and forces RLS on every user-owned table", func(t *testing.T) {
		rows, err := migrationDB.QueryContext(ctx, `
			SELECT relname, relrowsecurity, relforcerowsecurity
			FROM pg_class
			WHERE relnamespace = 'public'::regnamespace
				AND relname IN (
					'surfer_profiles', 'surf_spots', 'favorites',
					'boards', 'surfer_preferences', 'spot_interpretations',
					'spot_swell_exposures', 'spot_wind_responses',
					'spot_interpretation_limitations', 'spot_forecast_mappings',
					'spot_forecast_mapping_points'
				)
			ORDER BY relname
		`)
		if err != nil {
			t.Fatalf("query RLS state: %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var table string
			var enabled, forced bool
			if err := rows.Scan(&table, &enabled, &forced); err != nil {
				t.Fatalf("scan RLS state: %v", err)
			}
			if !enabled || !forced {
				t.Errorf("%s RLS = enabled:%t forced:%t, want true/true", table, enabled, forced)
			}
			count++
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate RLS state: %v", err)
		}
		if count != 11 {
			t.Fatalf("RLS table count = %d, want 11", count)
		}
	})

	t.Run("fails closed without a transaction principal", func(t *testing.T) {
		var count int
		if err := applicationDB.QueryRowContext(ctx, "SELECT count(*) FROM surf_spots").Scan(&count); err != nil {
			t.Fatalf("query without principal: %v", err)
		}
		if count != 0 {
			t.Fatalf("visible rows without principal = %d, want 0", count)
		}

		_, err := applicationDB.ExecContext(ctx, `
			INSERT INTO surfer_profiles (owner_id, experience_level, display_units)
			VALUES ($1, 'intermediate', 'metric')
		`, principalA)
		if err == nil {
			t.Fatal("insert without principal succeeded, want RLS denial")
		}
	})

	createOwnedData(t, ctx, applicationDB, principalA, spotA, "A spot", 0)
	createOwnedData(t, ctx, applicationDB, principalB, spotB, "B spot", 1)
	seedForecastPoint(t, ctx, migrationDB)
	createRecommendationConfiguration(t, ctx, applicationDB, principalA, spotA)

	t.Run("isolates reads by the transaction principal", func(t *testing.T) {
		tx := beginPrincipalTransaction(t, ctx, applicationDB, principalA)
		defer tx.Rollback()

		rows, err := tx.QueryContext(ctx, "SELECT id::text FROM surf_spots ORDER BY id")
		if err != nil {
			t.Fatalf("query owned spots: %v", err)
		}
		defer rows.Close()

		var got []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatalf("scan spot: %v", err)
			}
			got = append(got, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate spots: %v", err)
		}
		if !slices.Equal(got, []string{spotA}) {
			t.Fatalf("visible spots = %v, want [%s]", got, spotA)
		}
	})

	t.Run("rejects cross-owner writes and favorite relationships", func(t *testing.T) {
		expectPrincipalStatementFailure(t, ctx, applicationDB, principalA, `
			INSERT INTO surf_spots (id, owner_id, name, position, time_zone)
			VALUES ($1, $2, 'Impersonated', ST_SetSRID(ST_Point(-9.2, 38.7), 4326)::geography, 'Europe/Lisbon')
		`, "cross-owner spot insert", "cccccccc-cccc-4ccc-8ccc-cccccccccccc", principalB)

		expectPrincipalStatementFailure(t, ctx, applicationDB, principalA, `
			INSERT INTO favorites (owner_id, spot_id, sort_position)
			VALUES ($1, $2, 2)
		`, "cross-owner favorite insert", principalA, spotB)
	})

	t.Run("isolates recommendation configuration by owner", func(t *testing.T) {
		tx := beginPrincipalTransaction(t, ctx, applicationDB, principalA)
		defer tx.Rollback()

		for _, table := range []string{
			"boards",
			"surfer_preferences",
			"spot_interpretations",
			"spot_swell_exposures",
			"spot_wind_responses",
			"spot_interpretation_limitations",
			"spot_forecast_mappings",
			"spot_forecast_mapping_points",
		} {
			var count int
			if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}
			if count != 1 {
				t.Fatalf("visible %s rows = %d, want 1", table, count)
			}
		}

		other := beginPrincipalTransaction(t, ctx, applicationDB, principalB)
		defer other.Rollback()
		var count int
		if err := other.QueryRowContext(ctx, "SELECT count(*) FROM boards").Scan(&count); err != nil {
			t.Fatalf("count other-owner boards: %v", err)
		}
		if count != 0 {
			t.Fatalf("other owner saw %d boards, want 0", count)
		}
	})

	t.Run("keeps recommendation configuration unavailable to ingestion", func(t *testing.T) {
		var canReadMappings, canReadPreferences bool
		if err := migrationDB.QueryRowContext(ctx, `
			SELECT
				has_table_privilege('the_search_ingester', 'public.spot_forecast_mappings', 'SELECT'),
				has_table_privilege('the_search_ingester', 'public.surfer_preferences', 'SELECT')
		`).Scan(&canReadMappings, &canReadPreferences); err != nil {
			t.Fatalf("query recommendation ingestion privileges: %v", err)
		}
		if canReadMappings || canReadPreferences {
			t.Fatalf("ingester recommendation privileges = mappings:%t preferences:%t, want false/false", canReadMappings, canReadPreferences)
		}
	})

	t.Run("enforces recommendation configuration constraints", func(t *testing.T) {
		expectPrincipalStatementFailure(t, ctx, applicationDB, principalA, `
			UPDATE surfer_preferences
			SET preferred_wave_height_max_mm = comfort_wave_height_max_mm + 1
			WHERE owner_id = $1
		`, "preference outside comfort envelope", principalA)

		expectPrincipalStatementFailure(t, ctx, applicationDB, principalA, `
			UPDATE spot_swell_exposures
			SET full_circle = true, start_millidegrees = 1, end_millidegrees = 1
			WHERE owner_id = $1
		`, "ambiguous full-circle sector", principalA)

		expectPrincipalStatementFailure(t, ctx, applicationDB, principalA, `
			UPDATE spot_forecast_mappings
			SET valid_until = valid_from
			WHERE owner_id = $1
		`, "empty mapping validity interval", principalA)

		expectPrincipalCommitFailure(t, ctx, applicationDB, principalA, `
			UPDATE spot_forecast_mapping_points
			SET weight_basis_points = 9999
			WHERE owner_id = $1
		`, "mapping weights below 10000", principalA)

		expectPrincipalCommitFailure(t, ctx, applicationDB, principalA, `
			DELETE FROM spot_wind_responses WHERE owner_id = $1
		`, "spot interpretation without wind response", principalA)
	})

	t.Run("resets principal scope when a transaction ends", func(t *testing.T) {
		tx := beginPrincipalTransaction(t, ctx, applicationDB, principalA)
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit principal transaction: %v", err)
		}

		var count int
		if err := applicationDB.QueryRowContext(ctx, "SELECT count(*) FROM surf_spots").Scan(&count); err != nil {
			t.Fatalf("query after transaction: %v", err)
		}
		if count != 0 {
			t.Fatalf("visible rows after transaction = %d, want 0", count)
		}
	})

	t.Run("enforces canonical profile and spot values", func(t *testing.T) {
		expectPrincipalStatementFailure(t, ctx, applicationDB, principalA, `
			UPDATE surfer_profiles SET experience_level = 'legend' WHERE owner_id = $1
		`, "invalid experience level", principalA)

		expectPrincipalStatementFailure(t, ctx, applicationDB, principalA, `
			UPDATE surf_spots SET name = '   ' WHERE id = $1
		`, "blank spot name", spotA)
	})
}

func openRequiredDatabase(t *testing.T, key string) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv(key)
	if databaseURL == "" {
		t.Fatalf("%s is required for integration tests", key)
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database from %s: %v", key, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database from %s: %v", key, err)
		}
	})
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping database from %s: %v", key, err)
	}
	return db
}

func seedPrincipals(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	if _, err := db.ExecContext(ctx, `
		INSERT INTO principals (id) VALUES ($1), ($2)
	`, principalA, principalB); err != nil {
		t.Fatalf("seed principals: %v", err)
	}
}

func createOwnedData(t *testing.T, ctx context.Context, db *sql.DB, principalID, spotID, name string, sortPosition int) {
	t.Helper()

	tx := beginPrincipalTransaction(t, ctx, db, principalID)
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO surfer_profiles (owner_id, experience_level, display_units)
		VALUES ($1, 'intermediate', 'metric')
	`, principalID); err != nil {
		t.Fatalf("insert profile for %s: %v", principalID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO surf_spots (id, owner_id, name, position, time_zone)
		VALUES ($1, $2, $3, ST_SetSRID(ST_Point(-9.2, 38.7), 4326)::geography, 'Europe/Lisbon')
	`, spotID, principalID, name); err != nil {
		t.Fatalf("insert spot for %s: %v", principalID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO favorites (owner_id, spot_id, sort_position)
		VALUES ($1, $2, $3)
	`, principalID, spotID, sortPosition); err != nil {
		t.Fatalf("insert favorite for %s: %v", principalID, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit data for %s: %v", principalID, err)
	}
}

func seedForecastPoint(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO forecast_points (id, provider_id, position, selection_algorithm_version, active)
		VALUES (
			'33333333-3333-4333-8333-333333333333',
			'open-meteo',
			ST_SetSRID(ST_Point(-9.3, 38.8), 4326)::geography,
			'nearest-sea-grid-v1',
			false
		)
	`); err != nil {
		t.Fatalf("seed forecast point: %v", err)
	}
}

func createRecommendationConfiguration(t *testing.T, ctx context.Context, db *sql.DB, principalID, spotID string) {
	t.Helper()
	tx := beginPrincipalTransaction(t, ctx, db, principalID)
	defer tx.Rollback()

	statements := []string{
		`INSERT INTO boards (
			id, owner_id, name,
			usable_wave_height_min_mm, usable_wave_height_max_mm,
			usable_swell_period_min_ms, usable_swell_period_max_ms
		) VALUES ('44444444-4444-4444-8444-444444444444', $1, 'Daily driver', 500, 2500, 6000, 18000)`,
		`INSERT INTO surfer_preferences (
			id, owner_id, scope, spot_id,
			comfort_wave_height_min_mm, comfort_wave_height_max_mm,
			preferred_wave_height_min_mm, preferred_wave_height_max_mm,
			comfort_swell_period_min_ms, comfort_swell_period_max_ms,
			preferred_swell_period_min_ms, preferred_swell_period_max_ms,
			maximum_comfort_wind_mmps, preferred_maximum_wind_mmps
		) VALUES (
			'55555555-5555-4555-8555-555555555555', $1, 'global', NULL,
			500, 2500, 800, 1800, 6000, 18000, 9000, 14000, 12000, 7000
		)`,
		`INSERT INTO spot_interpretations (
			owner_id, spot_id,
			supported_wave_height_min_mm, supported_wave_height_max_mm,
			supported_swell_period_min_ms, supported_swell_period_max_ms,
			confidence
		) VALUES ($1, $2, 300, 3000, 6000, 20000, 'low')`,
		`INSERT INTO spot_swell_exposures (
			owner_id, spot_id, ordinal, start_millidegrees, end_millidegrees, full_circle, exposure_basis_points
		) VALUES ($1, $2, 0, 330000, 30000, false, 8000)`,
		`INSERT INTO spot_wind_responses (
			owner_id, spot_id, ordinal, start_millidegrees, end_millidegrees, full_circle, relationship
		) VALUES ($1, $2, 0, 0, 0, true, 'offshore')`,
		`INSERT INTO spot_interpretation_limitations (owner_id, spot_id, ordinal, limitation)
		 VALUES ($1, $2, 0, 'Uncalibrated offshore grid input')`,
		`INSERT INTO spot_forecast_mappings (owner_id, spot_id, valid_from)
		 VALUES ($1, $2, '2026-10-10T00:00:00Z')`,
		`INSERT INTO spot_forecast_mapping_points (
			owner_id, spot_id, forecast_point_id, role, weight_basis_points, selection_provenance
		) VALUES (
			$1, $2, '33333333-3333-4333-8333-333333333333', 'primary', 10000, 'nearest-sea-grid-v1'
		)`,
	}

	for index, statement := range statements {
		args := []any{principalID, spotID}
		if index < 2 {
			args = []any{principalID}
		}
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			t.Fatalf("insert recommendation configuration statement %d: %v", index, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit recommendation configuration: %v", err)
	}
}

func beginPrincipalTransaction(t *testing.T, ctx context.Context, db *sql.DB, principalID string) *sql.Tx {
	t.Helper()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.principal_id', $1, true)", principalID); err != nil {
		tx.Rollback()
		t.Fatalf("set principal: %v", err)
	}
	return tx
}

func expectPrincipalStatementFailure(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	principalID string,
	statement string,
	description string,
	args ...any,
) {
	t.Helper()

	tx := beginPrincipalTransaction(t, ctx, db, principalID)
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, statement, args...); err == nil {
		t.Fatalf("%s succeeded, want database rejection", description)
	}
}

func expectPrincipalCommitFailure(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	principalID string,
	statement string,
	description string,
	args ...any,
) {
	t.Helper()

	tx := beginPrincipalTransaction(t, ctx, db, principalID)
	if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
		tx.Rollback()
		t.Fatalf("%s failed before deferred constraints: %v", description, err)
	}
	if err := tx.Commit(); err == nil {
		t.Fatalf("%s committed, want deferred constraint rejection", description)
	}
}

func TestIntegrationConfigurationIsExplicit(t *testing.T) {
	for _, key := range []string{"TEST_MIGRATION_DATABASE_URL", "TEST_APPLICATION_DATABASE_URL"} {
		t.Run(fmt.Sprintf("requires %s", key), func(t *testing.T) {
			if os.Getenv(key) == "" {
				t.Fatalf("%s is required for integration tests", key)
			}
		})
	}
}
