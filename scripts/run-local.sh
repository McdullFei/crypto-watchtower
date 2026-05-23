#!/usr/bin/env bash
set -euo pipefail

docker compose -f deployments/docker-compose.yml up -d postgres redis
docker run --rm \
  -e CONFIG_PATH=/app/configs/config.example.yaml \
  -e CW_POSTGRES_DSN="postgres://postgres:postgres@host.docker.internal:5432/crypto_watchtower?sslmode=disable" \
  -e CW_REDIS_ADDR="host.docker.internal:6379" \
  -e CW_TELEGRAM_BOT_TOKEN="${CW_TELEGRAM_BOT_TOKEN:-YOUR_BOT_TOKEN}" \
  -e CW_TELEGRAM_DEFAULT_CHAT_ID="${CW_TELEGRAM_DEFAULT_CHAT_ID:-YOUR_CHAT_ID}" \
  -e CW_API_BEARER_TOKEN="${CW_API_BEARER_TOKEN:-change-me}" \
  -p "${APP_HTTP_PORT:-8080}:8080" \
  -v "$PWD":/app \
  -w /app \
  golang:1.24 \
  go run ./cmd/server
