#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENVIRONMENT="${1:-local}"
ACTION="${2:-up}"
ENV_FILE="${ROOT_DIR}/deployments/.env.${ENVIRONMENT}"
COMPOSE_FILE="${ROOT_DIR}/deployments/docker-compose.yml"

if [[ "${ENVIRONMENT}" != "local" && "${ENVIRONMENT}" != "test" && "${ENVIRONMENT}" != "prod" ]]; then
  echo "usage: $0 <local|test|prod> <up|down|restart|logs|ps>" >&2
  exit 1
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing env file: ${ENV_FILE}" >&2
  exit 1
fi

case "${ACTION}" in
  up)
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d
    ;;
  down)
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" down
    ;;
  restart)
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" down
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d
    ;;
  logs)
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" logs -f --tail=200
    ;;
  ps)
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" ps
    ;;
  *)
    echo "usage: $0 <local|test|prod> <up|down|restart|logs|ps>" >&2
    exit 1
    ;;
esac
