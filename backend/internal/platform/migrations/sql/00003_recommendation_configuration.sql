-- +goose Up

CREATE TABLE boards (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES principals (id) ON DELETE CASCADE,
    name text NOT NULL,
    usable_wave_height_min_mm integer NOT NULL,
    usable_wave_height_max_mm integer NOT NULL,
    usable_swell_period_min_ms integer NOT NULL,
    usable_swell_period_max_ms integer NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT boards_owned_key UNIQUE (id, owner_id),
    CONSTRAINT boards_name_check
        CHECK (name = btrim(name) AND char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT boards_wave_height_check
        CHECK (usable_wave_height_min_mm >= 0
            AND usable_wave_height_max_mm >= usable_wave_height_min_mm),
    CONSTRAINT boards_swell_period_check
        CHECK (usable_swell_period_min_ms > 0
            AND usable_swell_period_max_ms >= usable_swell_period_min_ms),
    CONSTRAINT boards_version_check CHECK (version > 0),
    CONSTRAINT boards_timestamps_check CHECK (updated_at >= created_at)
);

CREATE INDEX boards_owner_name_idx ON boards (owner_id, name, id);

CREATE TABLE surfer_preferences (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES principals (id) ON DELETE CASCADE,
    scope text NOT NULL,
    spot_id uuid,
    comfort_wave_height_min_mm integer NOT NULL,
    comfort_wave_height_max_mm integer NOT NULL,
    preferred_wave_height_min_mm integer NOT NULL,
    preferred_wave_height_max_mm integer NOT NULL,
    comfort_swell_period_min_ms integer NOT NULL,
    comfort_swell_period_max_ms integer NOT NULL,
    preferred_swell_period_min_ms integer NOT NULL,
    preferred_swell_period_max_ms integer NOT NULL,
    maximum_comfort_wind_mmps integer NOT NULL,
    preferred_maximum_wind_mmps integer NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT surfer_preferences_owned_spot_fk
        FOREIGN KEY (spot_id, owner_id)
        REFERENCES surf_spots (id, owner_id)
        ON DELETE CASCADE,
    CONSTRAINT surfer_preferences_scope_check CHECK (
        (scope = 'global' AND spot_id IS NULL)
        OR (scope = 'spot' AND spot_id IS NOT NULL)
    ),
    CONSTRAINT surfer_preferences_wave_height_check CHECK (
        comfort_wave_height_min_mm >= 0
        AND comfort_wave_height_max_mm >= comfort_wave_height_min_mm
        AND preferred_wave_height_min_mm >= comfort_wave_height_min_mm
        AND preferred_wave_height_max_mm >= preferred_wave_height_min_mm
        AND preferred_wave_height_max_mm <= comfort_wave_height_max_mm
    ),
    CONSTRAINT surfer_preferences_swell_period_check CHECK (
        comfort_swell_period_min_ms > 0
        AND comfort_swell_period_max_ms >= comfort_swell_period_min_ms
        AND preferred_swell_period_min_ms >= comfort_swell_period_min_ms
        AND preferred_swell_period_max_ms >= preferred_swell_period_min_ms
        AND preferred_swell_period_max_ms <= comfort_swell_period_max_ms
    ),
    CONSTRAINT surfer_preferences_wind_check CHECK (
        maximum_comfort_wind_mmps >= 0
        AND preferred_maximum_wind_mmps >= 0
        AND preferred_maximum_wind_mmps <= maximum_comfort_wind_mmps
    ),
    CONSTRAINT surfer_preferences_version_check CHECK (version > 0),
    CONSTRAINT surfer_preferences_timestamps_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX surfer_preferences_one_global_per_owner_idx
    ON surfer_preferences (owner_id) WHERE scope = 'global';
CREATE UNIQUE INDEX surfer_preferences_one_override_per_spot_idx
    ON surfer_preferences (owner_id, spot_id) WHERE scope = 'spot';

CREATE TABLE spot_interpretations (
    owner_id uuid NOT NULL,
    spot_id uuid NOT NULL,
    supported_wave_height_min_mm integer NOT NULL,
    supported_wave_height_max_mm integer NOT NULL,
    supported_swell_period_min_ms integer NOT NULL,
    supported_swell_period_max_ms integer NOT NULL,
    confidence text NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    PRIMARY KEY (owner_id, spot_id),
    CONSTRAINT spot_interpretations_owned_spot_fk
        FOREIGN KEY (spot_id, owner_id)
        REFERENCES surf_spots (id, owner_id)
        ON DELETE CASCADE,
    CONSTRAINT spot_interpretations_wave_height_check CHECK (
        supported_wave_height_min_mm >= 0
        AND supported_wave_height_max_mm >= supported_wave_height_min_mm
    ),
    CONSTRAINT spot_interpretations_swell_period_check CHECK (
        supported_swell_period_min_ms > 0
        AND supported_swell_period_max_ms >= supported_swell_period_min_ms
    ),
    CONSTRAINT spot_interpretations_confidence_check
        CHECK (confidence IN ('low', 'medium', 'high')),
    CONSTRAINT spot_interpretations_version_check CHECK (version > 0),
    CONSTRAINT spot_interpretations_timestamps_check CHECK (updated_at >= created_at)
);

CREATE TABLE spot_swell_exposures (
    owner_id uuid NOT NULL,
    spot_id uuid NOT NULL,
    ordinal integer NOT NULL,
    start_millidegrees integer NOT NULL,
    end_millidegrees integer NOT NULL,
    full_circle boolean NOT NULL DEFAULT false,
    exposure_basis_points integer NOT NULL,
    PRIMARY KEY (owner_id, spot_id, ordinal),
    CONSTRAINT spot_swell_exposures_interpretation_fk
        FOREIGN KEY (owner_id, spot_id)
        REFERENCES spot_interpretations (owner_id, spot_id)
        ON DELETE CASCADE,
    CONSTRAINT spot_swell_exposures_ordinal_check CHECK (ordinal >= 0),
    CONSTRAINT spot_swell_exposures_direction_check CHECK (
        start_millidegrees BETWEEN 0 AND 359999
        AND end_millidegrees BETWEEN 0 AND 359999
        AND (
            (full_circle AND start_millidegrees = 0 AND end_millidegrees = 0)
            OR (
                NOT full_circle
                AND NOT (start_millidegrees = 0 AND end_millidegrees = 359999)
            )
        )
    ),
    CONSTRAINT spot_swell_exposures_weight_check
        CHECK (exposure_basis_points BETWEEN 0 AND 10000)
);

CREATE TABLE spot_wind_responses (
    owner_id uuid NOT NULL,
    spot_id uuid NOT NULL,
    ordinal integer NOT NULL,
    start_millidegrees integer NOT NULL,
    end_millidegrees integer NOT NULL,
    full_circle boolean NOT NULL DEFAULT false,
    relationship text NOT NULL,
    PRIMARY KEY (owner_id, spot_id, ordinal),
    CONSTRAINT spot_wind_responses_interpretation_fk
        FOREIGN KEY (owner_id, spot_id)
        REFERENCES spot_interpretations (owner_id, spot_id)
        ON DELETE CASCADE,
    CONSTRAINT spot_wind_responses_ordinal_check CHECK (ordinal >= 0),
    CONSTRAINT spot_wind_responses_direction_check CHECK (
        start_millidegrees BETWEEN 0 AND 359999
        AND end_millidegrees BETWEEN 0 AND 359999
        AND (
            (full_circle AND start_millidegrees = 0 AND end_millidegrees = 0)
            OR (
                NOT full_circle
                AND NOT (start_millidegrees = 0 AND end_millidegrees = 359999)
            )
        )
    ),
    CONSTRAINT spot_wind_responses_relationship_check
        CHECK (relationship IN ('offshore', 'cross_shore', 'onshore', 'sheltered'))
);

CREATE TABLE spot_interpretation_limitations (
    owner_id uuid NOT NULL,
    spot_id uuid NOT NULL,
    ordinal integer NOT NULL,
    limitation text NOT NULL,
    PRIMARY KEY (owner_id, spot_id, ordinal),
    CONSTRAINT spot_interpretation_limitations_interpretation_fk
        FOREIGN KEY (owner_id, spot_id)
        REFERENCES spot_interpretations (owner_id, spot_id)
        ON DELETE CASCADE,
    CONSTRAINT spot_interpretation_limitations_ordinal_check CHECK (ordinal BETWEEN 0 AND 19),
    CONSTRAINT spot_interpretation_limitations_text_check
        CHECK (limitation = btrim(limitation) AND char_length(limitation) BETWEEN 1 AND 300)
);

CREATE TABLE spot_forecast_mappings (
    owner_id uuid NOT NULL,
    spot_id uuid NOT NULL,
    valid_from timestamptz NOT NULL,
    valid_until timestamptz,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    PRIMARY KEY (owner_id, spot_id),
    CONSTRAINT spot_forecast_mappings_owned_spot_fk
        FOREIGN KEY (spot_id, owner_id)
        REFERENCES surf_spots (id, owner_id)
        ON DELETE CASCADE,
    CONSTRAINT spot_forecast_mappings_validity_check
        CHECK (valid_until IS NULL OR valid_until > valid_from),
    CONSTRAINT spot_forecast_mappings_version_check CHECK (version > 0),
    CONSTRAINT spot_forecast_mappings_timestamps_check CHECK (updated_at >= created_at)
);

CREATE TABLE spot_forecast_mapping_points (
    owner_id uuid NOT NULL,
    spot_id uuid NOT NULL,
    forecast_point_id uuid NOT NULL REFERENCES forecast_points (id),
    role text NOT NULL,
    weight_basis_points integer NOT NULL,
    selection_provenance text NOT NULL,
    PRIMARY KEY (owner_id, spot_id, forecast_point_id),
    CONSTRAINT spot_forecast_mapping_points_mapping_fk
        FOREIGN KEY (owner_id, spot_id)
        REFERENCES spot_forecast_mappings (owner_id, spot_id)
        ON DELETE CASCADE,
    CONSTRAINT spot_forecast_mapping_points_role_check
        CHECK (role IN ('primary', 'secondary')),
    CONSTRAINT spot_forecast_mapping_points_weight_check
        CHECK (weight_basis_points BETWEEN 1 AND 10000),
    CONSTRAINT spot_forecast_mapping_points_provenance_check
        CHECK (selection_provenance = btrim(selection_provenance)
            AND char_length(selection_provenance) BETWEEN 1 AND 200)
);

CREATE UNIQUE INDEX spot_forecast_mapping_one_primary_idx
    ON spot_forecast_mapping_points (owner_id, spot_id) WHERE role = 'primary';

-- Child collections are replaced inside one application transaction. Deferred
-- constraint triggers validate the final aggregate rather than rejecting the
-- temporarily incomplete state between header and child statements.
-- +goose StatementBegin
CREATE FUNCTION app_private.enforce_spot_interpretation_complete()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
DECLARE
    target_owner uuid := COALESCE(NEW.owner_id, OLD.owner_id);
    target_spot uuid := COALESCE(NEW.spot_id, OLD.spot_id);
    swell_count integer;
    wind_count integer;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM public.spot_interpretations
        WHERE owner_id = target_owner AND spot_id = target_spot
    ) THEN
        RETURN NULL;
    END IF;

    SELECT count(*) INTO swell_count
    FROM public.spot_swell_exposures
    WHERE owner_id = target_owner AND spot_id = target_spot;

    SELECT count(*) INTO wind_count
    FROM public.spot_wind_responses
    WHERE owner_id = target_owner AND spot_id = target_spot;

    IF swell_count = 0 OR wind_count = 0 THEN
        RAISE EXCEPTION 'spot interpretation requires swell exposure and wind response'
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION app_private.enforce_spot_interpretation_complete() FROM PUBLIC;

CREATE CONSTRAINT TRIGGER spot_interpretations_complete_header_trigger
AFTER INSERT OR UPDATE ON spot_interpretations
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION app_private.enforce_spot_interpretation_complete();

CREATE CONSTRAINT TRIGGER spot_interpretations_complete_swell_trigger
AFTER INSERT OR UPDATE OR DELETE ON spot_swell_exposures
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION app_private.enforce_spot_interpretation_complete();

CREATE CONSTRAINT TRIGGER spot_interpretations_complete_wind_trigger
AFTER INSERT OR UPDATE OR DELETE ON spot_wind_responses
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION app_private.enforce_spot_interpretation_complete();

-- +goose StatementBegin
CREATE FUNCTION app_private.enforce_spot_forecast_mapping_complete()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
DECLARE
    target_owner uuid := COALESCE(NEW.owner_id, OLD.owner_id);
    target_spot uuid := COALESCE(NEW.spot_id, OLD.spot_id);
    point_count integer;
    primary_count integer;
    total_weight integer;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM public.spot_forecast_mappings
        WHERE owner_id = target_owner AND spot_id = target_spot
    ) THEN
        RETURN NULL;
    END IF;

    SELECT count(*), count(*) FILTER (WHERE role = 'primary'), COALESCE(sum(weight_basis_points), 0)
    INTO point_count, primary_count, total_weight
    FROM public.spot_forecast_mapping_points
    WHERE owner_id = target_owner AND spot_id = target_spot;

    IF point_count = 0 OR primary_count != 1 OR total_weight != 10000 THEN
        RAISE EXCEPTION 'spot forecast mapping requires points, one primary, and weight 10000'
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION app_private.enforce_spot_forecast_mapping_complete() FROM PUBLIC;

CREATE CONSTRAINT TRIGGER spot_forecast_mappings_complete_header_trigger
AFTER INSERT OR UPDATE ON spot_forecast_mappings
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION app_private.enforce_spot_forecast_mapping_complete();

CREATE CONSTRAINT TRIGGER spot_forecast_mappings_complete_points_trigger
AFTER INSERT OR UPDATE OR DELETE ON spot_forecast_mapping_points
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION app_private.enforce_spot_forecast_mapping_complete();

REVOKE ALL ON TABLE
    boards,
    surfer_preferences,
    spot_interpretations,
    spot_swell_exposures,
    spot_wind_responses,
    spot_interpretation_limitations,
    spot_forecast_mappings,
    spot_forecast_mapping_points
FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
    boards,
    surfer_preferences,
    spot_interpretations,
    spot_swell_exposures,
    spot_wind_responses,
    spot_interpretation_limitations,
    spot_forecast_mappings,
    spot_forecast_mapping_points
TO the_search_app;

ALTER TABLE boards ENABLE ROW LEVEL SECURITY;
ALTER TABLE boards FORCE ROW LEVEL SECURITY;
CREATE POLICY boards_owner_policy ON boards
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE surfer_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE surfer_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY surfer_preferences_owner_policy ON surfer_preferences
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE spot_interpretations ENABLE ROW LEVEL SECURITY;
ALTER TABLE spot_interpretations FORCE ROW LEVEL SECURITY;
CREATE POLICY spot_interpretations_owner_policy ON spot_interpretations
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE spot_swell_exposures ENABLE ROW LEVEL SECURITY;
ALTER TABLE spot_swell_exposures FORCE ROW LEVEL SECURITY;
CREATE POLICY spot_swell_exposures_owner_policy ON spot_swell_exposures
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE spot_wind_responses ENABLE ROW LEVEL SECURITY;
ALTER TABLE spot_wind_responses FORCE ROW LEVEL SECURITY;
CREATE POLICY spot_wind_responses_owner_policy ON spot_wind_responses
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE spot_interpretation_limitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE spot_interpretation_limitations FORCE ROW LEVEL SECURITY;
CREATE POLICY spot_interpretation_limitations_owner_policy ON spot_interpretation_limitations
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE spot_forecast_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE spot_forecast_mappings FORCE ROW LEVEL SECURITY;
CREATE POLICY spot_forecast_mappings_owner_policy ON spot_forecast_mappings
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE spot_forecast_mapping_points ENABLE ROW LEVEL SECURITY;
ALTER TABLE spot_forecast_mapping_points FORCE ROW LEVEL SECURITY;
CREATE POLICY spot_forecast_mapping_points_owner_policy ON spot_forecast_mapping_points
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());
