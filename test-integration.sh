#!/bin/bash
set -e

cleanup() {
  docker compose down 2>/dev/null || true
}
trap cleanup EXIT

# Find an available port starting from 1883
find_port() {
  local port=1883
  while ss -tlnp | grep -q ":${port} "; do
    port=$((port + 1))
    if [ "$port" -gt 65535 ]; then
      echo "ERROR: no available port found" >&2
      exit 1
    fi
  done
  echo "$port"
}

MQTT_PORT=$(find_port)
export MQTT_PORT

echo "Using MQTT port: $MQTT_PORT"
echo "Starting MQTT broker..."
docker compose up -d --wait

echo "Running integration tests..."
go test -tags=integration -v -timeout 60s -count=1 ./...

echo "All integration tests passed."
