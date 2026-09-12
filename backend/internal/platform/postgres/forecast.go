package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonriber/the-search-surf/backend/internal/application/forecasting"
	"github.com/jonriber/the-search-surf/backend/internal/forecast"
)

// ForecastRepository persists provider-neutral batches with the dedicated
// ingestion role. It never needs principal or private surf-spot access.
type ForecastRepository struct {
	pool *pgxpool.Pool
}

// NewForecastRepository constructs forecast storage over an ingestion-role pool.
func NewForecastRepository(pool *pgxpool.Pool) (*ForecastRepository, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &ForecastRepository{pool: pool}, nil
}

// ListActivePoints returns public provider sampling coordinates, never private spots.
func (repository *ForecastRepository) ListActivePoints(ctx context.Context, providerID string) ([]forecast.Point, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, ST_X(position::geometry), ST_Y(position::geometry)
		FROM forecast_points
		WHERE provider_id = $1 AND active
		ORDER BY id
	`, providerID)
	if err != nil {
		return nil, fmt.Errorf("select active forecast points: %w", err)
	}
	defer rows.Close()

	points := make([]forecast.Point, 0)
	for rows.Next() {
		var reference string
		var longitude, latitude float64
		if err := rows.Scan(&reference, &longitude, &latitude); err != nil {
			return nil, fmt.Errorf("scan active forecast point: %w", err)
		}
		point, err := forecast.NewPoint(reference, longitude, latitude)
		if err != nil {
			return nil, fmt.Errorf("restore active forecast point: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active forecast points: %w", err)
	}
	return points, nil
}

// Save atomically inserts a batch and returns false for an idempotent replay.
func (repository *ForecastRepository) Save(ctx context.Context, request forecasting.FetchRequest, batch forecast.Batch) (bool, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin forecast transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batchID := uuid.New()
	idempotencyKey := forecastIdempotencyKey(request, batch)
	err = tx.QueryRow(ctx, `
		INSERT INTO forecast_batches (
			id, provider_id, window_starts_at, window_ends_at, fetched_at,
			transformation_version, attribution_text, attribution_url,
			licence_name, licence_url, idempotency_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id
	`,
		batchID,
		batch.ProviderID,
		request.StartsAt,
		request.EndsAt,
		batch.FetchedAt,
		batch.TransformationVersion,
		batch.Attribution.Text,
		batch.Attribution.URL,
		batch.Attribution.License,
		batch.Attribution.LicenseURL,
		idempotencyKey[:],
	).Scan(&batchID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit forecast replay: %w", err)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert forecast batch: %w", err)
	}

	for _, series := range batch.Series {
		pointID, err := uuid.Parse(series.PointReference)
		if err != nil {
			return false, fmt.Errorf("parse forecast point reference %q: %w", series.PointReference, err)
		}
		for _, source := range series.Sources {
			var issuedAt any
			if source.IssueTimeKnown {
				issuedAt = source.IssuedAt
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO forecast_sources (
					batch_id, forecast_point_id, component, model_reference,
					issued_at, issue_time_known, available_at, sampled_position,
					native_temporal_resolution_seconds
				)
				VALUES (
					$1, $2, $3, $4, $5, $6, $7,
					ST_SetSRID(ST_Point($8, $9), 4326)::geography, $10
				)
			`,
				batchID,
				pointID,
				source.Component,
				source.ModelReference,
				issuedAt,
				source.IssueTimeKnown,
				source.AvailableAt,
				source.SampledLongitude,
				source.SampledLatitude,
				int64(source.NativeTemporalResolution/time.Second),
			); err != nil {
				return false, fmt.Errorf("insert forecast source: %w", err)
			}
		}
		for _, hour := range series.Hours {
			if _, err := tx.Exec(ctx, `
				INSERT INTO forecast_observations (
					batch_id, forecast_point_id, valid_at,
					wave_height_metres, wave_direction_degrees, wave_period_seconds,
					swell_height_metres, swell_direction_degrees, swell_period_seconds,
					wind_speed_metres_per_second, wind_direction_degrees,
					wind_gust_metres_per_second, sea_level_height_metres_above_msl
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			`,
				batchID,
				pointID,
				hour.ValidAt,
				nullableMeasurement(hour.WaveHeightMetres),
				nullableMeasurement(hour.WaveDirectionDegrees),
				nullableMeasurement(hour.WavePeriodSeconds),
				nullableMeasurement(hour.SwellHeightMetres),
				nullableMeasurement(hour.SwellDirectionDegrees),
				nullableMeasurement(hour.SwellPeriodSeconds),
				nullableMeasurement(hour.WindSpeedMetresPerSecond),
				nullableMeasurement(hour.WindDirectionDegrees),
				nullableMeasurement(hour.WindGustMetresPerSecond),
				nullableMeasurement(hour.SeaLevelHeightMetresAboveMSL),
			); err != nil {
				return false, fmt.Errorf("insert forecast observation: %w", err)
			}
		}
	}

	for _, digest := range batch.PayloadDigests {
		if _, err := tx.Exec(ctx, `
			INSERT INTO forecast_payloads (batch_id, component, sha256)
			VALUES ($1, $2, $3)
		`, batchID, digest.Component, digest.SHA256); err != nil {
			return false, fmt.Errorf("insert forecast payload digest: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit forecast transaction: %w", err)
	}
	return true, nil
}

// Quarantine persists checksum-only evidence for a malformed provider payload.
func (repository *ForecastRepository) Quarantine(ctx context.Context, rejection forecasting.Rejection) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO forecast_rejections (
			id, provider_id, forecast_point_reference, component, fetched_at,
			sha256, storage_reference, reason
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, NULLIF($7, ''), $8)
	`,
		uuid.New(),
		rejection.ProviderID,
		rejection.PointReference,
		rejection.Component,
		rejection.FetchedAt,
		rejection.SHA256,
		rejection.StorageReference,
		rejection.Reason,
	)
	if err != nil {
		return fmt.Errorf("insert forecast rejection: %w", err)
	}
	return nil
}

// ReserveProviderAttempt atomically reserves one physical request against both
// daily and monthly UTC limits. A failed reservation rolls back both counters.
func (repository *ForecastRepository) ReserveProviderAttempt(
	ctx context.Context,
	providerID string,
	accountScope string,
	at time.Time,
	dailyLimit int64,
	monthlyLimit int64,
) (bool, error) {
	if dailyLimit < 1 || monthlyLimit < 1 {
		return false, errors.New("provider quota limits must be positive")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin quota transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	utc := at.UTC()
	periods := []struct {
		kind  string
		start time.Time
		limit int64
	}{
		{kind: "day", start: time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC), limit: dailyLimit},
		{kind: "month", start: time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC), limit: monthlyLimit},
	}
	for _, period := range periods {
		var used int64
		err := tx.QueryRow(ctx, `
			INSERT INTO provider_quota_usage (
				provider_id, account_scope, period_kind, period_started_at, used_units
			)
			VALUES ($1, $2, $3, $4, 1)
			ON CONFLICT (provider_id, account_scope, period_kind, period_started_at)
			DO UPDATE SET
				used_units = provider_quota_usage.used_units + 1,
				updated_at = transaction_timestamp()
			WHERE provider_quota_usage.used_units < $5
			RETURNING used_units
		`, providerID, accountScope, period.kind, period.start, period.limit).Scan(&used)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("reserve %s provider quota: %w", period.kind, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit quota reservation: %w", err)
	}
	return true, nil
}

func nullableMeasurement(measurement forecast.Measurement) any {
	if !measurement.Available {
		return nil
	}
	return measurement.Value
}

func forecastIdempotencyKey(request forecasting.FetchRequest, batch forecast.Batch) [sha256.Size]byte {
	hasher := sha256.New()
	writeHashField(hasher, batch.ProviderID)
	writeHashField(hasher, request.StartsAt.UTC().Format(time.RFC3339Nano))
	writeHashField(hasher, request.EndsAt.UTC().Format(time.RFC3339Nano))
	writeHashField(hasher, batch.TransformationVersion)

	points := append([]forecast.Point(nil), request.Points...)
	slices.SortFunc(points, func(left, right forecast.Point) int { return compareStrings(left.Reference, right.Reference) })
	for _, point := range points {
		writeHashField(hasher, point.Reference)
		writeHashFloat(hasher, point.Longitude)
		writeHashFloat(hasher, point.Latitude)
	}

	digests := append([]forecast.PayloadDigest(nil), batch.PayloadDigests...)
	slices.SortFunc(digests, func(left, right forecast.PayloadDigest) int {
		return compareStrings(string(left.Component), string(right.Component))
	})
	for _, digest := range digests {
		writeHashField(hasher, string(digest.Component))
		writeHashField(hasher, digest.SHA256)
	}

	seriesValues := append([]forecast.Series(nil), batch.Series...)
	slices.SortFunc(seriesValues, func(left, right forecast.Series) int {
		return compareStrings(left.PointReference, right.PointReference)
	})
	for _, series := range seriesValues {
		writeHashField(hasher, series.PointReference)
		sources := append([]forecast.Source(nil), series.Sources...)
		slices.SortFunc(sources, func(left, right forecast.Source) int {
			return compareStrings(string(left.Component), string(right.Component))
		})
		for _, source := range sources {
			writeHashField(hasher, string(source.Component))
			writeHashField(hasher, source.ModelReference)
			writeHashField(hasher, source.IssuedAt.UTC().Format(time.RFC3339Nano))
			writeHashField(hasher, source.AvailableAt.UTC().Format(time.RFC3339Nano))
			writeHashFloat(hasher, source.SampledLongitude)
			writeHashFloat(hasher, source.SampledLatitude)
			writeHashField(hasher, source.NativeTemporalResolution.String())
		}
	}

	var key [sha256.Size]byte
	copy(key[:], hasher.Sum(nil))
	return key
}

func writeHashField(hasher hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = hasher.Write(length[:])
	_, _ = hasher.Write([]byte(value))
}

func writeHashFloat(hasher hash.Hash, value float64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], math.Float64bits(value))
	_, _ = hasher.Write(encoded[:])
}

func compareStrings(left, right string) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
