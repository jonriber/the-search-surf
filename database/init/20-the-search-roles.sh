#!/bin/sh

set -eu

: "${THE_SEARCH_MIGRATOR_PASSWORD:?THE_SEARCH_MIGRATOR_PASSWORD is required}"
: "${THE_SEARCH_APPLICATION_PASSWORD:?THE_SEARCH_APPLICATION_PASSWORD is required}"
: "${THE_SEARCH_INGESTER_PASSWORD:?THE_SEARCH_INGESTER_PASSWORD is required}"

if [ -n "${POSTGRES_HOST:-}" ]; then
  set -- --host "${POSTGRES_HOST}"
else
  set --
fi

psql "$@" \
  --set ON_ERROR_STOP=1 \
  --set migrator_password="${THE_SEARCH_MIGRATOR_PASSWORD}" \
  --set application_password="${THE_SEARCH_APPLICATION_PASSWORD}" \
  --set ingester_password="${THE_SEARCH_INGESTER_PASSWORD}" \
  --username "${POSTGRES_USER}" \
  --dbname "${POSTGRES_DB}" <<'SQL'
SELECT 'CREATE ROLE the_search_migrator' WHERE NOT EXISTS (
    SELECT FROM pg_roles WHERE rolname = 'the_search_migrator'
) \gexec
ALTER ROLE the_search_migrator
    WITH LOGIN PASSWORD :'migrator_password'
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS;

SELECT 'CREATE ROLE the_search_app' WHERE NOT EXISTS (
    SELECT FROM pg_roles WHERE rolname = 'the_search_app'
) \gexec
ALTER ROLE the_search_app
    WITH LOGIN PASSWORD :'application_password'
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS;

SELECT 'CREATE ROLE the_search_ingester' WHERE NOT EXISTS (
    SELECT FROM pg_roles WHERE rolname = 'the_search_ingester'
) \gexec
ALTER ROLE the_search_ingester
    WITH LOGIN PASSWORD :'ingester_password'
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS;

REVOKE CONNECT ON DATABASE the_search FROM PUBLIC;
GRANT CONNECT ON DATABASE the_search TO the_search_migrator, the_search_app, the_search_ingester;
GRANT CREATE ON DATABASE the_search TO the_search_migrator;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO the_search_migrator;
GRANT USAGE ON SCHEMA public TO the_search_app;
GRANT USAGE ON SCHEMA public TO the_search_ingester;

ALTER DATABASE the_search SET timezone TO 'UTC';
ALTER ROLE the_search_migrator SET statement_timeout TO '2min';
ALTER ROLE the_search_app SET statement_timeout TO '15s';
ALTER ROLE the_search_app SET lock_timeout TO '3s';
ALTER ROLE the_search_app SET idle_in_transaction_session_timeout TO '30s';
ALTER ROLE the_search_ingester SET statement_timeout TO '30s';
ALTER ROLE the_search_ingester SET lock_timeout TO '3s';
ALTER ROLE the_search_ingester SET idle_in_transaction_session_timeout TO '30s';
SQL
