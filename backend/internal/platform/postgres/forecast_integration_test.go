//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
	"github.com/jonriber/the-search-surf/backend/internal/forecast"
	"github.com/jonriber/the-search-surf/backend/internal/platform/postgres"
)

func TestForecastPersistenceContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	migrationConnection, err := pgx.Connect(ctx, requiredEnvironment(t, "TEST_MIGRATION_DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect migration database: %v", err)
	}
	defer migrationConnection.Close(context.Background())

	pointID := uuid.New()
	if _, err := migrationConnection.Exec(ctx, `
		INSERT INTO forecast_points (
			id, provider_id, position, selection_algorithm_version
		)
		VALUES (
			$1, 'open-meteo',
			ST_SetSRID(ST_Point(-9.417, 38.963), 4326)::geography,
			'nearest-sea-grid-v1'
		)
	`, pointID); err != nil {
		t.Fatalf("seed forecast point: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = migrationConnection.Exec(cleanupCtx, "DELETE FROM forecast_rejections WHERE provider_id = 'open-meteo'")
		_, _ = migrationConnection.Exec(cleanupCtx, "DELETE FROM forecast_batches WHERE provider_id = 'open-meteo'")
		_, _ = migrationConnection.Exec(cleanupCtx, "DELETE FROM provider_quota_usage WHERE provider_id = 'open-meteo'")
		_, _ = migrationConnection.Exec(cleanupCtx, "DELETE FROM forecast_points WHERE id = $1", pointID)
	})

	ingesterPool, err := pgxpool.New(ctx, requiredEnvironment(t, "TEST_INGESTER_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create ingester pool: %v", err)
	}
	defer ingesterPool.Close()
	repository, err := postgres.NewForecastRepository(ingesterPool)
	if err != nil {
		t.Fatal(err)
	}

	points, err := repository.ListActivePoints(ctx, "open-meteo")
	if err != nil || len(points) != 1 || points[0].Reference != pointID.String() {
		t.Fatalf("ListActivePoints() = (%+v, %v)", points, err)
	}
	request, err := forecasting.NewFetchRequest(
		points,
		time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC),
		time.Date(2026, time.September, 12, 14, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	batch := persistenceBatch(request)
	if err := forecasting.ValidateBatch(request, batch, "open-meteo"); err != nil {
		t.Fatalf("validate fixture: %v", err)
	}

	persisted, err := repository.Save(ctx, request, batch)
	if err != nil || !persisted {
		t.Fatalf("Save() = (%t, %v), want true/nil", persisted, err)
	}
	persisted, err = repository.Save(ctx, request, batch)
	if err != nil || persisted {
		t.Fatalf("replayed Save() = (%t, %v), want false/nil", persisted, err)
	}

	var batches, sources, observations, payloads int
	if err := migrationConnection.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM forecast_batches WHERE provider_id = 'open-meteo'),
			(SELECT count(*) FROM forecast_sources WHERE forecast_point_id = $1),
			(SELECT count(*) FROM forecast_observations WHERE forecast_point_id = $1),
			(SELECT count(*) FROM forecast_payloads)
	`, pointID).Scan(&batches, &sources, &observations, &payloads); err != nil {
		t.Fatalf("query persisted forecast graph: %v", err)
	}
	if batches != 1 || sources != 3 || observations != 2 || payloads != 3 {
		t.Fatalf("persisted counts = batches:%d sources:%d observations:%d payloads:%d", batches, sources, observations, payloads)
	}

	rejection := forecasting.Rejection{
		ProviderID:     "open-meteo",
		PointReference: pointID.String(),
		Component:      forecast.ComponentWaves,
		FetchedAt:      request.StartsAt,
		SHA256:         repeatHex('d'),
		Reason:         "mismatched hourly arrays",
	}
	if err := repository.Quarantine(ctx, rejection); err != nil {
		t.Fatalf("Quarantine() error = %v", err)
	}
	var reason, digest string
	if err := migrationConnection.QueryRow(ctx, `
		SELECT reason, sha256
		FROM forecast_rejections
		WHERE provider_id = 'open-meteo'
	`).Scan(&reason, &digest); err != nil {
		t.Fatalf("query quarantine record: %v", err)
	}
	if reason != rejection.Reason || digest != rejection.SHA256 {
		t.Fatalf("quarantine = reason:%q digest:%q", reason, digest)
	}
}

func TestProviderQuotaReservationIsAtomicAndBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	migrationConnection, err := pgx.Connect(ctx, requiredEnvironment(t, "TEST_MIGRATION_DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect migration database: %v", err)
	}
	defer migrationConnection.Close(context.Background())
	ingesterPool, err := pgxpool.New(ctx, requiredEnvironment(t, "TEST_INGESTER_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create ingester pool: %v", err)
	}
	defer ingesterPool.Close()
	repository, err := postgres.NewForecastRepository(ingesterPool)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = migrationConnection.Exec(context.Background(), `
			DELETE FROM provider_quota_usage
			WHERE provider_id = 'quota-test'
		`)
	})

	now := time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC)
	for attempt, want := range []bool{true, true, false} {
		reserved, err := repository.ReserveProviderAttempt(ctx, "quota-test", "free", now, 2, 10)
		if err != nil || reserved != want {
			t.Fatalf("reservation %d = (%t, %v), want %t/nil", attempt+1, reserved, err, want)
		}
	}
	var daily, monthly int64
	if err := migrationConnection.QueryRow(ctx, `
		SELECT
			max(used_units) FILTER (WHERE period_kind = 'day'),
			max(used_units) FILTER (WHERE period_kind = 'month')
		FROM provider_quota_usage
		WHERE provider_id = 'quota-test'
	`).Scan(&daily, &monthly); err != nil {
		t.Fatalf("query quota usage: %v", err)
	}
	if daily != 2 || monthly != 2 {
		t.Fatalf("quota usage = day:%d month:%d, want 2/2", daily, monthly)
	}
}

func TestForecastDatabaseConstraintsRejectImplausibleValues(t *testing.T) {
	ctx := context.Background()
	migrationConnection, err := pgx.Connect(ctx, requiredEnvironment(t, "TEST_MIGRATION_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer migrationConnection.Close(ctx)

	tx, err := migrationConnection.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	pointID := uuid.New()
	batchID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO forecast_points (id, provider_id, position, selection_algorithm_version)
		VALUES ($1, 'constraint-test', ST_SetSRID(ST_Point(0, 0), 4326)::geography, 'test-v1')
	`, pointID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO forecast_batches (
			id, provider_id, window_starts_at, window_ends_at, fetched_at,
			transformation_version, attribution_text, attribution_url,
			licence_name, licence_url, idempotency_key
		)
		VALUES (
			$1, 'constraint-test', '2026-09-12T12:00:00Z', '2026-09-12T13:00:00Z',
			'2026-09-12T12:05:00Z', 'test-v1', 'test attribution', 'https://example.com',
			'test licence', 'https://example.com/licence', $2
		)
	`, batchID, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO forecast_observations (
			batch_id, forecast_point_id, valid_at, wave_height_metres
		)
		VALUES ($1, $2, '2026-09-12T12:00:00Z', -0.1)
	`, batchID, pointID)
	if err == nil {
		t.Fatal("implausible observation insert succeeded")
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.ConstraintName != "forecast_observations_wave_height_check" {
		t.Fatalf("constraint error = %v", err)
	}
}

func persistenceBatch(request forecasting.FetchRequest) forecast.Batch {
	issuedAt := request.StartsAt.Add(-6 * time.Hour)
	availableAt := issuedAt.Add(90 * time.Minute)
	sources := make([]forecast.Source, 0, 3)
	for _, component := range []forecast.Component{forecast.ComponentWaves, forecast.ComponentWind, forecast.ComponentSeaLevel} {
		sources = append(sources, forecast.Source{
			Component:                component,
			ModelReference:           "explicit-model",
			IssuedAt:                 issuedAt,
			AvailableAt:              availableAt,
			IssueTimeKnown:           true,
			SampledLongitude:         -9.42,
			SampledLatitude:          38.96,
			NativeTemporalResolution: time.Hour,
		})
	}
	hours := make([]forecast.HourlyConditions, 0, 2)
	for offset := range 2 {
		hours = append(hours, forecast.HourlyConditions{
			ValidAt:                      request.StartsAt.Add(time.Duration(offset) * time.Hour),
			WaveHeightMetres:             measurement(1.7),
			WaveDirectionDegrees:         measurement(287),
			WavePeriodSeconds:            measurement(11),
			SwellHeightMetres:            measurement(1.3),
			SwellDirectionDegrees:        measurement(292),
			SwellPeriodSeconds:           measurement(13),
			WindSpeedMetresPerSecond:     measurement(4.2),
			WindDirectionDegrees:         measurement(45),
			WindGustMetresPerSecond:      measurement(6.4),
			SeaLevelHeightMetresAboveMSL: measurement(0.6),
		})
	}
	return forecast.Batch{
		ProviderID:            "open-meteo",
		FetchedAt:             request.StartsAt.Add(5 * time.Minute),
		TransformationVersion: "open-meteo-v1",
		Attribution: forecast.Attribution{
			Text:       "Weather data by Open-Meteo.com",
			URL:        "https://open-meteo.com/",
			License:    "CC BY 4.0",
			LicenseURL: "https://creativecommons.org/licenses/by/4.0/",
		},
		PayloadDigests: []forecast.PayloadDigest{
			{Component: forecast.ComponentWaves, SHA256: repeatHex('a')},
			{Component: forecast.ComponentWind, SHA256: repeatHex('b')},
			{Component: forecast.ComponentSeaLevel, SHA256: repeatHex('c')},
		},
		Series: []forecast.Series{{
			PointReference: request.Points[0].Reference,
			Sources:        sources,
			Hours:          hours,
		}},
	}
}

func measurement(value float64) forecast.Measurement {
	return forecast.Measurement{Value: value, Available: true}
}

func repeatHex(character byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = character
	}
	return string(result)
}
