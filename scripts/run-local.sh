#!/usr/bin/env bash
set -euo pipefail

docker compose -f deployments/docker-compose.yml up -d
docker run --rm \
  -v "$PWD":/app \
  -w /app \
  golang:1.24 \
  go run ./cmd/server
