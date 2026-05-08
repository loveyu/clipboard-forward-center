中文 | [English](README.md)

# Clipboard Forward Center

一个基于 MQTT 的轻量级剪贴板消息转发服务，使用 Go 编写。支持设备间剪贴板内容同步、去重过滤，并提供 HTTP API 进行消息存储。

## 功能特性

- 基于可配置规则在 MQTT 主题间转发剪贴板消息
- 基于 SHA256 哈希的去重过滤，支持可配置的时间窗口
- HTTP API 消息存储，使用 Bearer Token 认证
- 内存消息存储，支持 TTL 和可配置容量
- MQTT 自动重连
- 通过 GitHub Actions 实现多平台自动构建

## 快速开始

```bash
# 构建
./build.sh v1.0.0

# 编辑配置
cp config.yaml myconfig.yaml
# 根据你的 MQTT broker 和客户端设置编辑 myconfig.yaml

# 运行
CONFIG_PATH=myconfig.yaml ./clipboard-forward-center start

# 开启 DEBUG 日志
DEBUG=1 CONFIG_PATH=myconfig.yaml ./clipboard-forward-center start
```

## 配置

完整示例见 [config.yaml](config.yaml)。

### DSN

MQTT 连接字符串格式：
```
mqtt://用户名:密码@主机:端口?clientId=客户端ID&connectTimeout=3&keepAliveInterval=20&automaticReconnect=true&reconnectMaxInterval=60
```

### 转发规则

每条规则定义源主题（`from`）和目标主题（`to`）：

```yaml
forward:
  - from: ["clipboard-in-text/mobile-k50"]
    to: ["clipboard-out-text/work-min-debian"]
    type: text
    format: json
    contentField: content
```

- `type`：内容类型标识（用于去重哈希计算）
- `format`：消息负载格式（`json` 或 `yaml`）
- `contentField`：负载中用于去重过滤的字段名

### 去重过滤

过滤器在时间窗口内阻止转发重复内容：
- 哈希 = SHA256(type + ":" + contentField的值)
- 如果目标客户端最近发送或接收过相同哈希的消息，则跳过转发

```yaml
filter:
  time: 5s  # 支持 ms/s/m/h，支持小数
```

### 存储

HTTP 消息存储设置：

```yaml
storage:
  maxRecords: 100  # 最大存储消息数
  expire: 10m      # 消息过期时间
```

### 客户端

每个客户端拥有名称和 Token，用于 HTTP API 认证：

```yaml
clients:
  - name: mobile-k50
    token: ABCD123456
```

## HTTP API

### 写入消息

```
PUT|POST /client/{client}/{msgId}
Authorization: Bearer <token>
Content-Type: application/octet-stream

<消息内容>
```

URL 中的 `client` 必须与 Token 关联的客户端名称一致。

### 读取消息

```
GET /client/{client}/{msgId}
Authorization: Bearer <token>
```

任意有效 Token 均可读取任意客户端的消息（公共读权限）。

## CLI 命令

| 命令 | 说明 |
|------|------|
| `start` | 启动服务（默认命令） |
| `help` | 显示帮助信息 |
| `download-config` | 从 `REMOTE_CONFIG_URL` 下载配置 |
| `version` | 显示版本号 |

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `CONFIG_PATH` | 配置文件路径 | `config.yaml` |
| `DEBUG` | 启用 DEBUG 日志 | （未设置） |
| `REMOTE_CONFIG_URL` | `download-config` 命令使用的远程配置 URL | （未设置） |

## 构建

```bash
./build.sh [版本号]   # 本地构建
./test.sh             # 运行测试
```

推送 tag（`v*`）会触发 GitHub Actions 自动构建 Linux（amd64/arm64）、macOS（amd64/arm64）和 Windows（amd64）平台的二进制文件，并创建 GitHub Release。

## 许可证

Apache 2.0
