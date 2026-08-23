#!/usr/bin/env bash

set -Eeuo pipefail

readonly project_name="the-search-smoke"

cleanup() {
  docker compose --project-name "${project_name}" down --remove-orphans
}

trap cleanup EXIT INT TERM

docker compose --project-name "${project_name}" up --build --detach --wait --wait-timeout 120

curl --fail --silent --show-error http://127.0.0.1:8080/health/live >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8081/health/live >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8081/api/health/ready >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8081/api/version | grep --quiet '"version":"local"'

echo "Compose smoke test passed."
