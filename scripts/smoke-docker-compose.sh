#!/usr/bin/env bash
set -euo pipefail

# Runs the local Docker Compose smoke checks for the Phase 1.5 closure loop.
#
# Author: __AUTHOR__
# Date: 2026-06-29

ENV_FILE="${ENV_FILE:-deployments/.env.local}"
COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.yml}"
APP_HTTP_PORT="${APP_HTTP_PORT:-18080}"
API_TOKEN="${CW_API_BEARER_TOKEN:-change-me}"
BASE_URL="http://127.0.0.1:${APP_HTTP_PORT}"

echo "Checking Docker Compose config..."
docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" config >/dev/null

echo "Starting CryptoWatchtower stack on port ${APP_HTTP_PORT}..."
APP_HTTP_PORT="${APP_HTTP_PORT}" docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d --build

echo "Waiting for /health..."
for attempt in $(seq 1 60); do
  if curl -fsS "${BASE_URL}/health" >/tmp/crypto-watchtower-health.json; then
    break
  fi
  if [ "${attempt}" -eq 60 ]; then
    echo "Timed out waiting for ${BASE_URL}/health" >&2
    exit 1
  fi
  sleep 2
done

echo "Checking read APIs..."
curl -fsS "${BASE_URL}/api/v1/rules" >/tmp/crypto-watchtower-rules.json
curl -fsS -H "Authorization: Bearer ${API_TOKEN}" "${BASE_URL}/api/v1/admin/events?limit=5" >/tmp/crypto-watchtower-events.json

echo "Checking write API authentication..."
status="$(curl -sS -o /tmp/crypto-watchtower-unauthorized.json -w "%{http_code}" -X POST "${BASE_URL}/api/v1/rules")"
if [ "${status}" != "401" ]; then
  echo "Expected unauthorized rule write to return 401, got ${status}" >&2
  exit 1
fi

echo "Smoke checks passed."
