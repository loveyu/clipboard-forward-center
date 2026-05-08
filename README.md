[中文](README_CN.md) | English

# Clipboard Forward Center

A lightweight MQTT-based clipboard message forwarding service written in Go. It synchronizes clipboard content between devices with dedup filtering and provides an HTTP API for message storage.

## Features

- Forward clipboard messages between MQTT topics based on configurable rules
- SHA256 hash-based dedup filtering with configurable time window
- HTTP API for message storage with Bearer token authentication
- In-memory message store with TTL and configurable capacity
- Automatic MQTT reconnection
- Multi-platform builds via GitHub Actions

## Quick Start

```bash
# Build
./build.sh v1.0.0

# Edit config
cp config.yaml myconfig.yaml
# Edit myconfig.yaml with your MQTT broker and client settings

# Run
CONFIG_PATH=myconfig.yaml ./clipboard-forward-center start

# With debug logging
DEBUG=1 CONFIG_PATH=myconfig.yaml ./clipboard-forward-center start
```

## Configuration

See [config.yaml](config.yaml) for a full example.

### DSN

MQTT connection string format:
```
mqtt://user:password@host:port?clientId=id&connectTimeout=3&keepAliveInterval=20&automaticReconnect=true&reconnectMaxInterval=60
```

### Forward Rules

Each rule defines source topics (`from`) and destination topics (`to`):

```yaml
forward:
  - from: ["clipboard-in-text/mobile-k50"]
    to: ["clipboard-out-text/work-min-debian"]
    type: text
    format: json
    contentField: content
```

- `type`: Content type identifier (used for dedup hash computation)
- `format`: Message payload format (`json` or `yaml`)
- `contentField`: Field name in the payload to extract for dedup filtering

### Dedup Filter

The filter prevents forwarding duplicate content within a time window:
- Hash = SHA256(type + ":" + contentField_value)
- If the target client has recently sent or received a message with the same hash, the forward is skipped

```yaml
filter:
  time: 5s  # supports ms/s/m/h, including decimals
```

### Storage

HTTP message storage settings:

```yaml
storage:
  maxRecords: 100  # max stored messages
  expire: 10m      # message TTL
```

### Clients

Each client has a name and token for HTTP API authentication:

```yaml
clients:
  - name: mobile-k50
    token: ABCD123456
```

## HTTP API

### Write Message

```
PUT|POST /client/{client}/{msgId}
Authorization: Bearer <token>
Content-Type: application/octet-stream

<message body>
```

The `client` in the URL must match the token's associated client name.

### Read Message

```
GET /client/{client}/{msgId}
Authorization: Bearer <token>
```

Any valid token can read any client's messages (public read access).

## CLI Commands

| Command | Description |
|---------|-------------|
| `start` | Start the service (default) |
| `help` | Show help message |
| `download-config` | Download config from `REMOTE_CONFIG_URL` |
| `version` | Show version |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFIG_PATH` | Config file path | `config.yaml` |
| `DEBUG` | Enable debug logging | (unset) |
| `REMOTE_CONFIG_URL` | URL for `download-config` command | (unset) |

## Building

```bash
./build.sh [version]   # Local build
./test.sh              # Run tests
```

Pushing a tag (`v*`) triggers GitHub Actions to build binaries for Linux (amd64/arm64), macOS (amd64/arm64), and Windows (amd64), and creates a GitHub Release.

## License

Apache 2.0
