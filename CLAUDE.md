# CLAUDE.md

## Project Overview

Clipboard message forwarding center - a Go service that forwards MQTT clipboard messages between devices with dedup filtering and HTTP message storage.

## Tech Stack

- Go 1.26.3
- MQTT (paho.mqtt.golang) for message forwarding
- YAML config (yaml.v3)
- HTTP API for message storage

## Project Structure

```
main.go                          # Entry point, CLI commands
internal/
  config/config.go               # Config types, YAML parsing, DSN parsing
  store/store.go                 # In-memory message store with TTL
  filter/filter.go               # SHA256 hash dedup filter
  forward/engine.go              # MQTT message forwarding engine
  mqttclient/client.go           # MQTT client wrapper
  httpserver/server.go           # HTTP API server with Bearer auth
```

## Commands

- `./build.sh [version]` - Build binary (default version: "dev")
- `./test.sh` - Run all tests
- `go run . start` - Start service
- `go run . help` - Show help
- `go run . download-config` - Download config from REMOTE_CONFIG_URL

## Key Design

- MQTT messages are forwarded based on `forward` rules in config
- Dedup filter uses SHA256 hash of (type + contentField) with configurable time window
- HTTP PUT/POST requires client name to match Bearer token; GET allows any valid token
- In-memory store with configurable max records and TTL

## Config

Default path: `config.yaml` (override with `CONFIG_PATH` env).
DSN format: `mqtt://user:pass@host:port?clientId=xxx&connectTimeout=3&keepAliveInterval=20`
