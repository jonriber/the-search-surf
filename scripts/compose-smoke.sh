#!/usr/bin/env bash

set -Eeuo pipefail

readonly project_name="the-search-smoke"
readonly database_port="55433"

cleanup() {
	docker compose --project-name "${project_name}" down --volumes --remove-orphans
}

trap cleanup EXIT INT TERM

export THE_SEARCH_DATABASE_PORT="${database_port}"

docker compose --project-name "${project_name}" up --build --detach --wait --wait-timeout 120

docker compose --project-name "${project_name}" run --rm migrate version | grep --quiet '"current_version":1'
docker compose --project-name "${project_name}" run --rm bootstrap | grep --quiet '"status":"existing"'

curl --fail --silent --show-error http://127.0.0.1:8080/health/live >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8081/health/live >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8081/api/health/ready >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8081/api/version | grep --quiet '"version":"local"'

echo "Compose smoke test passed."
