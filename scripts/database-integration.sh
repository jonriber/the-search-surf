#!/usr/bin/env bash

set -Eeuo pipefail

readonly project_name="the-search-database-test"
readonly database_port="55432"
readonly admin_password="integration-admin-only"
readonly migrator_password="integration-migrator-only"
readonly application_password="integration-app-only"

cleanup() {
  docker compose --project-name "${project_name}" down --volumes --remove-orphans
}

trap cleanup EXIT INT TERM

export THE_SEARCH_DATABASE_PORT="${database_port}"
export THE_SEARCH_DATABASE_ADMIN_PASSWORD="${admin_password}"
export THE_SEARCH_DATABASE_MIGRATOR_PASSWORD="${migrator_password}"
export THE_SEARCH_DATABASE_APPLICATION_PASSWORD="${application_password}"

docker compose --project-name "${project_name}" up --detach --wait --wait-timeout 120 database

export TEST_MIGRATION_DATABASE_URL="postgres://the_search_migrator:${migrator_password}@127.0.0.1:${database_port}/the_search?sslmode=disable"
export TEST_APPLICATION_DATABASE_URL="postgres://the_search_app:${application_password}@127.0.0.1:${database_port}/the_search?sslmode=disable"

make -C backend test-integration
