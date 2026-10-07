-- +goose Up

CREATE TABLE forecast_points (
    id uuid PRIMARY KEY,
    provider_id text NOT NULL,
    position geography(Point, 4326) NOT NULL,
    selection_algorithm_version text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT forecast_points_provider_check
        CHECK (provider_id = btrim(provider_id) AND char_length(provider_id) BETWEEN 1 AND 64),
    CONSTRAINT forecast_points_selection_version_check
        CHECK (selection_algorithm_version = btrim(selection_algorithm_version)
            AND char_length(selection_algorithm_version) BETWEEN 1 AND 64),
    CONSTRAINT forecast_points_timestamps_check CHECK (updated_at >= created_at)
);

CREATE INDEX forecast_points_position_gist_idx ON forecast_points USING gist (position);
CREATE INDEX forecast_points_active_provider_idx ON forecast_points (provider_id, id) WHERE active;

CREATE TABLE forecast_batches (
    id uuid PRIMARY KEY,
    provider_id text NOT NULL,
    window_starts_at timestamptz NOT NULL,
    window_ends_at timestamptz NOT NULL,
    fetched_at timestamptz NOT NULL,
    transformation_version text NOT NULL,
    attribution_text text NOT NULL,
    attribution_url text NOT NULL,
    licence_name text NOT NULL,
    licence_url text NOT NULL,
    idempotency_key bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT forecast_batches_provider_check
        CHECK (provider_id = btrim(provider_id) AND char_length(provider_id) BETWEEN 1 AND 64),
    CONSTRAINT forecast_batches_window_check
        CHECK (window_starts_at < window_ends_at
            AND date_trunc('hour', window_starts_at) = window_starts_at
            AND date_trunc('hour', window_ends_at) = window_ends_at),
    CONSTRAINT forecast_batches_transformation_version_check
        CHECK (transformation_version = btrim(transformation_version)
            AND char_length(transformation_version) BETWEEN 1 AND 64),
    CONSTRAINT forecast_batches_attribution_check
        CHECK (char_length(btrim(attribution_text)) BETWEEN 1 AND 200
            AND attribution_url ~ '^https://'
            AND char_length(btrim(licence_name)) BETWEEN 1 AND 100
            AND licence_url ~ '^https://'),
    CONSTRAINT forecast_batches_idempotency_key_check CHECK (octet_length(idempotency_key) = 32)
);

CREATE INDEX forecast_batches_provider_fetched_idx ON forecast_batches (provider_id, fetched_at DESC);

CREATE TABLE forecast_sources (
    batch_id uuid NOT NULL REFERENCES forecast_batches (id) ON DELETE CASCADE,
    forecast_point_id uuid NOT NULL REFERENCES forecast_points (id),
    component text NOT NULL,
    model_reference text NOT NULL,
    issued_at timestamptz,
    issue_time_known boolean NOT NULL,
    available_at timestamptz NOT NULL,
    sampled_position geography(Point, 4326) NOT NULL,
    native_temporal_resolution_seconds integer NOT NULL,
    PRIMARY KEY (batch_id, forecast_point_id, component),
    CONSTRAINT forecast_sources_component_check CHECK (component IN ('waves', 'wind', 'sea_level')),
    CONSTRAINT forecast_sources_model_check
        CHECK (model_reference = btrim(model_reference)
            AND char_length(model_reference) BETWEEN 1 AND 128),
    CONSTRAINT forecast_sources_issue_time_check CHECK (
        issue_time_known = (issued_at IS NOT NULL)
        AND (issued_at IS NULL OR available_at >= issued_at)
    ),
    CONSTRAINT forecast_sources_resolution_check CHECK (native_temporal_resolution_seconds > 0)
);

CREATE INDEX forecast_sources_point_issue_idx
    ON forecast_sources (forecast_point_id, component, issued_at DESC NULLS LAST);
CREATE INDEX forecast_sources_sampled_position_gist_idx ON forecast_sources USING gist (sampled_position);

CREATE TABLE forecast_observations (
    batch_id uuid NOT NULL REFERENCES forecast_batches (id) ON DELETE CASCADE,
    forecast_point_id uuid NOT NULL REFERENCES forecast_points (id),
    valid_at timestamptz NOT NULL,
    wave_height_metres double precision,
    wave_direction_degrees double precision,
    wave_period_seconds double precision,
    swell_height_metres double precision,
    swell_direction_degrees double precision,
    swell_period_seconds double precision,
    wind_speed_metres_per_second double precision,
    wind_direction_degrees double precision,
    wind_gust_metres_per_second double precision,
    sea_level_height_metres_above_msl double precision,
    PRIMARY KEY (batch_id, forecast_point_id, valid_at),
    CONSTRAINT forecast_observations_hour_check CHECK (date_trunc('hour', valid_at) = valid_at),
    CONSTRAINT forecast_observations_nonempty_check CHECK (num_nonnulls(
        wave_height_metres,
        wave_direction_degrees,
        wave_period_seconds,
        swell_height_metres,
        swell_direction_degrees,
        swell_period_seconds,
        wind_speed_metres_per_second,
        wind_direction_degrees,
        wind_gust_metres_per_second,
        sea_level_height_metres_above_msl
    ) > 0),
    CONSTRAINT forecast_observations_wave_height_check
        CHECK (wave_height_metres IS NULL OR wave_height_metres BETWEEN 0 AND 50),
    CONSTRAINT forecast_observations_wave_direction_check
        CHECK (wave_direction_degrees IS NULL OR wave_direction_degrees >= 0 AND wave_direction_degrees < 360),
    CONSTRAINT forecast_observations_wave_period_check
        CHECK (wave_period_seconds IS NULL OR wave_period_seconds > 0 AND wave_period_seconds <= 60),
    CONSTRAINT forecast_observations_swell_height_check
        CHECK (swell_height_metres IS NULL OR swell_height_metres BETWEEN 0 AND 50),
    CONSTRAINT forecast_observations_swell_direction_check
        CHECK (swell_direction_degrees IS NULL OR swell_direction_degrees >= 0 AND swell_direction_degrees < 360),
    CONSTRAINT forecast_observations_swell_period_check
        CHECK (swell_period_seconds IS NULL OR swell_period_seconds > 0 AND swell_period_seconds <= 60),
    CONSTRAINT forecast_observations_wind_speed_check
        CHECK (wind_speed_metres_per_second IS NULL OR wind_speed_metres_per_second BETWEEN 0 AND 150),
    CONSTRAINT forecast_observations_wind_direction_check
        CHECK (wind_direction_degrees IS NULL OR wind_direction_degrees >= 0 AND wind_direction_degrees < 360),
    CONSTRAINT forecast_observations_wind_gust_check
        CHECK (wind_gust_metres_per_second IS NULL OR wind_gust_metres_per_second BETWEEN 0 AND 150),
    CONSTRAINT forecast_observations_sea_level_check
        CHECK (sea_level_height_metres_above_msl IS NULL
            OR sea_level_height_metres_above_msl BETWEEN -20 AND 20)
);

CREATE INDEX forecast_observations_point_valid_idx
    ON forecast_observations (forecast_point_id, valid_at, batch_id);

CREATE TABLE forecast_payloads (
    batch_id uuid NOT NULL REFERENCES forecast_batches (id) ON DELETE CASCADE,
    component text NOT NULL,
    sha256 text NOT NULL,
    storage_reference text,
    PRIMARY KEY (batch_id, component, sha256),
    CONSTRAINT forecast_payloads_component_check CHECK (component IN ('waves', 'wind', 'sea_level')),
    CONSTRAINT forecast_payloads_sha256_check CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT forecast_payloads_storage_reference_check CHECK (
        storage_reference IS NULL
        OR storage_reference = btrim(storage_reference) AND char_length(storage_reference) BETWEEN 1 AND 500
    )
);

CREATE TABLE forecast_rejections (
    id uuid PRIMARY KEY,
    provider_id text NOT NULL,
    forecast_point_reference text,
    component text,
    fetched_at timestamptz NOT NULL,
    sha256 text NOT NULL,
    storage_reference text,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT forecast_rejections_provider_check
        CHECK (provider_id = btrim(provider_id) AND char_length(provider_id) BETWEEN 1 AND 64),
    CONSTRAINT forecast_rejections_point_reference_check CHECK (
        forecast_point_reference IS NULL
        OR forecast_point_reference = btrim(forecast_point_reference)
            AND char_length(forecast_point_reference) BETWEEN 1 AND 200
    ),
    CONSTRAINT forecast_rejections_component_check
        CHECK (component IS NULL OR component IN ('waves', 'wind', 'sea_level')),
    CONSTRAINT forecast_rejections_sha256_check CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT forecast_rejections_storage_reference_check CHECK (
        storage_reference IS NULL
        OR storage_reference = btrim(storage_reference) AND char_length(storage_reference) BETWEEN 1 AND 500
    ),
    CONSTRAINT forecast_rejections_reason_check
        CHECK (reason = btrim(reason) AND char_length(reason) BETWEEN 1 AND 500)
);

CREATE INDEX forecast_rejections_provider_fetched_idx
    ON forecast_rejections (provider_id, fetched_at DESC);

CREATE TABLE provider_quota_usage (
    provider_id text NOT NULL,
    account_scope text NOT NULL,
    period_kind text NOT NULL,
    period_started_at timestamptz NOT NULL,
    used_units bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    PRIMARY KEY (provider_id, account_scope, period_kind, period_started_at),
    CONSTRAINT provider_quota_usage_provider_check
        CHECK (provider_id = btrim(provider_id) AND char_length(provider_id) BETWEEN 1 AND 64),
    CONSTRAINT provider_quota_usage_account_check
        CHECK (account_scope = btrim(account_scope) AND char_length(account_scope) BETWEEN 1 AND 64),
    CONSTRAINT provider_quota_usage_period_kind_check CHECK (period_kind IN ('day', 'month')),
    CONSTRAINT provider_quota_usage_period_start_check CHECK (
        period_started_at = CASE period_kind
            WHEN 'day' THEN date_trunc('day', period_started_at)
            WHEN 'month' THEN date_trunc('month', period_started_at)
        END
    ),
    CONSTRAINT provider_quota_usage_units_check CHECK (used_units >= 0)
);

REVOKE ALL ON TABLE
    forecast_points,
    forecast_batches,
    forecast_sources,
    forecast_observations,
    forecast_payloads,
    forecast_rejections,
    provider_quota_usage
FROM PUBLIC;

GRANT SELECT ON TABLE
    forecast_points,
    forecast_batches,
    forecast_sources,
    forecast_observations
TO the_search_app;

GRANT SELECT ON TABLE forecast_points TO the_search_ingester;
GRANT SELECT, INSERT ON TABLE
    forecast_batches,
    forecast_sources,
    forecast_observations,
    forecast_payloads,
    forecast_rejections
TO the_search_ingester;
GRANT SELECT, INSERT, UPDATE ON TABLE provider_quota_usage TO the_search_ingester;
