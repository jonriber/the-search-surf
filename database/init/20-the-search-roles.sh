#!/bin/sh

set -eu

: "${THE_SEARCH_MIGRATOR_PASSWORD:?THE_SEARCH_MIGRATOR_PASSWORD is required}"
: "${THE_SEARCH_APPLICATION_PASSWORD:?THE_SEARCH_APPLICATION_PASSWORD is required}"

psql \
  --set ON_ERROR_STOP=1 \
  --set migrator_password="${THE_SEARCH_MIGRATOR_PASSWORD}" \
  --set application_password="${THE_SEARCH_APPLICATION_PASSWORD}" \
  --username "${POSTGRES_USER}" \
  --dbname "${POSTGRES_DB}" <<'SQL'
CREATE ROLE the_search_migrator
    LOGIN
    PASSWORD :'migrator_password'
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS;

CREATE ROLE the_search_app
    LOGIN
    PASSWORD :'application_password'
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS;

REVOKE CONNECT ON DATABASE the_search FROM PUBLIC;
GRANT CONNECT ON DATABASE the_search TO the_search_migrator, the_search_app;
GRANT CREATE ON DATABASE the_search TO the_search_migrator;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO the_search_migrator;
GRANT USAGE ON SCHEMA public TO the_search_app;

ALTER DATABASE the_search SET timezone TO 'UTC';
ALTER ROLE the_search_migrator SET statement_timeout TO '2min';
ALTER ROLE the_search_app SET statement_timeout TO '15s';
ALTER ROLE the_search_app SET lock_timeout TO '3s';
ALTER ROLE the_search_app SET idle_in_transaction_session_timeout TO '30s';
SQL
