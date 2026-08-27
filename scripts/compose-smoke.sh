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

profile_response="$(curl --fail --silent --show-error \
	--request POST \
	--header 'Content-Type: application/json' \
	--data '{"experienceLevel":"intermediate","displayUnits":"metric"}' \
	http://127.0.0.1:8081/api/profile)"
grep --quiet '"experienceLevel":"intermediate"' <<<"${profile_response}"
if grep --quiet 'ownerId' <<<"${profile_response}"; then
	echo "Profile response exposed its owner identifier." >&2
	exit 1
fi

spot_response="$(curl --fail --silent --show-error \
	--request POST \
	--header 'Content-Type: application/json' \
	--data '{"name":"Compose Test Spot","longitude":-9.1,"latitude":39.1,"timeZone":"Europe/Lisbon"}' \
	http://127.0.0.1:8081/api/spots)"
spot_id="$(sed -n 's/.*"id":"\([^"]*\)".*/\1/p' <<<"${spot_response}")"
test -n "${spot_id}"

curl --fail --silent --show-error \
	--request POST \
	--header 'Content-Type: application/json' \
	--data "{\"spotId\":\"${spot_id}\",\"sortPosition\":0}" \
	http://127.0.0.1:8081/api/favorites | grep --quiet '"sortPosition":0'
curl --fail --silent --show-error http://127.0.0.1:8081/api/favorites | grep --quiet "\"spotId\":\"${spot_id}\""

echo "Compose smoke test passed."
