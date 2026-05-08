#!/bin/bash
set -e

cleanup() {
  docker compose down 2>/dev/null || true
}
trap cleanup EXIT

echo "Starting MQTT broker..."
docker compose up -d --wait

echo "Running integration tests..."
go test -tags=integration -v -timeout 60s -count=1 ./...

echo "All integration tests passed."
