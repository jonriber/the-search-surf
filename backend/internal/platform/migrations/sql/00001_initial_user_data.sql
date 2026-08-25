-- +goose Up

CREATE SCHEMA app_private;
REVOKE ALL ON SCHEMA app_private FROM PUBLIC;
GRANT USAGE ON SCHEMA app_private TO the_search_app;

-- +goose StatementBegin
CREATE FUNCTION app_private.current_principal_id()
RETURNS uuid
LANGUAGE sql
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $$
    SELECT NULLIF(current_setting('app.principal_id', true), '')::uuid
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION app_private.current_principal_id() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app_private.current_principal_id() TO the_search_app;

CREATE TABLE principals (
    id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    disabled_at timestamptz,
    CONSTRAINT principals_disabled_after_creation_check
        CHECK (disabled_at IS NULL OR disabled_at >= created_at)
);

CREATE TABLE surfer_profiles (
    owner_id uuid PRIMARY KEY REFERENCES principals (id) ON DELETE CASCADE,
    experience_level text NOT NULL,
    display_units text NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT surfer_profiles_experience_level_check
        CHECK (experience_level IN ('beginner', 'intermediate', 'advanced', 'expert')),
    CONSTRAINT surfer_profiles_display_units_check
        CHECK (display_units IN ('metric', 'imperial')),
    CONSTRAINT surfer_profiles_version_check CHECK (version > 0),
    CONSTRAINT surfer_profiles_timestamps_check CHECK (updated_at >= created_at)
);

CREATE TABLE surf_spots (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES principals (id) ON DELETE CASCADE,
    name text NOT NULL,
    position geography(Point, 4326) NOT NULL,
    time_zone text NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT surf_spots_owned_key UNIQUE (id, owner_id),
    CONSTRAINT surf_spots_name_check
        CHECK (name = btrim(name) AND char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT surf_spots_time_zone_check
        CHECK (time_zone = btrim(time_zone) AND char_length(time_zone) BETWEEN 1 AND 255),
    CONSTRAINT surf_spots_version_check CHECK (version > 0),
    CONSTRAINT surf_spots_timestamps_check CHECK (updated_at >= created_at)
);

CREATE INDEX surf_spots_owner_name_idx ON surf_spots (owner_id, name, id);
CREATE INDEX surf_spots_position_gist_idx ON surf_spots USING gist (position);

CREATE TABLE favorites (
    owner_id uuid NOT NULL REFERENCES principals (id) ON DELETE CASCADE,
    spot_id uuid NOT NULL,
    sort_position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT transaction_timestamp(),
    PRIMARY KEY (owner_id, spot_id),
    CONSTRAINT favorites_owned_spot_fk
        FOREIGN KEY (spot_id, owner_id)
        REFERENCES surf_spots (id, owner_id)
        ON DELETE CASCADE,
    CONSTRAINT favorites_sort_position_check CHECK (sort_position >= 0),
    CONSTRAINT favorites_timestamps_check CHECK (updated_at >= created_at)
);

CREATE INDEX favorites_owner_order_idx ON favorites (owner_id, sort_position, spot_id);

REVOKE ALL ON TABLE principals, surfer_profiles, surf_spots, favorites FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE surfer_profiles, surf_spots, favorites TO the_search_app;

ALTER TABLE surfer_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE surfer_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY surfer_profiles_owner_policy ON surfer_profiles
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE surf_spots ENABLE ROW LEVEL SECURITY;
ALTER TABLE surf_spots FORCE ROW LEVEL SECURITY;
CREATE POLICY surf_spots_owner_policy ON surf_spots
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());

ALTER TABLE favorites ENABLE ROW LEVEL SECURITY;
ALTER TABLE favorites FORCE ROW LEVEL SECURITY;
CREATE POLICY favorites_owner_policy ON favorites
    USING (owner_id = app_private.current_principal_id())
    WITH CHECK (owner_id = app_private.current_principal_id());
