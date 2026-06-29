# CryptoWatchtower

CryptoWatchtower 是一个基于 Go 的实时币圈异动监控平台。当前阶段聚焦 **Binance / OKX 市场数据采集、异常规则判断、Telegram 告警推送、PostgreSQL/Redis 状态管理**，目标是先跑通稳定的实时监控链路，再逐步扩展 Dashboard、Discord、AI Summary 和 SaaS 能力。

> CryptoWatchtower only provides real-time market telemetry and alerting. It is not financial advice.
>
> CryptoWatchtower 仅提供实时市场数据监控和风险提醒，不构成任何投资建议。

## Current Status

当前 Phase 1 骨架已经包含：

- Go 模块化单体服务
- Binance Spot/Futures WebSocket collector
- Binance Futures Funding REST fetcher
- WebSocket 自动重连与指数退避
- `MarketEvent` 标准化模型
- 大单、爆仓、Funding 异常规则
- 60 秒窗口累计大单规则
- Telegram notifier，包含失败重试与退避
- Telegram Bot polling 命令：`/start`、`/status`、`/rules`、`/test`
- PostgreSQL migration runner
- Redis 去重与限流钩子
- `/health` 健康检查，包含 PostgreSQL、Redis、collector 状态
- Dockerfile 与 Docker Compose
- OKX 可选只读 collector：spot/swap trades、swap liquidation、swap funding-rate

暂未完成：

- 用户侧 Web Dashboard

当前已提供第一版后台管理台骨架：

- `/admin` 轻量后台页面
- 管理 API：概览、规则、告警、事件、通知日志
- 后台事件列表当前展示的是“告警相关事件”，不是 Binance 全量市场事件流
- 运营后台已包含运行状态、exchange/symbol/list 过滤、系统规则编辑和 24 小时趋势摘要

## Architecture

```text
Binance WS / REST, OKX Public WS / REST
  -> Collector
  -> EventBus
  -> Rule Engine
  -> Alert Pipeline
  -> Telegram Notifier

PostgreSQL:
  users / alert_rules / market_events / alerts / notification_logs / schema_migrations

Redis:
  dedupe keys / rule rate limit keys / short-lived state
```

默认启用 Binance：

- Spot `aggTrade`
- Futures `aggTrade`
- Futures `forceOrder`
- Futures Funding REST

OKX 当前为可选 collector，默认关闭：

- Spot `trades`
- Swap `trades`
- Swap `liquidation-orders`
- Swap `funding-rate`
- OKX swap notional 依赖 `GET /api/v5/public/instruments` 的 `ctVal`，不能把合约张数直接当币数量。
- OKX live `trades` 频道可能受交易等级限制；如果返回 `64003`，表示交易所权限限制，不是本地解析错误。

## Requirements

- Docker Desktop / Docker Engine
- Docker Compose
- Runtime image versions: Go `1.24`, PostgreSQL `16.14`, Redis `7.0.15`
- Optional: Go 1.24 if running without Docker

## Configuration

默认配置文件：

```text
configs/config.example.yaml
```

关键配置项：

| Key | Description |
| --- | --- |
| `binance.enabled` | Enable Binance collectors and funding fetcher |
| `binance.spot_ws_base_url` | Binance Spot WebSocket base URL |
| `binance.futures_ws_base_url` | Binance Futures WebSocket base URL |
| `binance.futures_rest_base_url` | Binance Futures REST base URL |
| `binance.symbols` | Monitored symbols |
| `okx.enabled` | Enable optional OKX public collector |
| `okx.public_ws_base_url` | OKX public WebSocket base URL |
| `okx.rest_base_url` | OKX public REST base URL |
| `okx.symbols` | OKX monitored symbols, using compact symbols such as `BTCUSDT` |
| `webhook.enabled` | Enable optional Discord-compatible webhook notifications |
| `webhook.url` | Webhook endpoint URL |
| `webhook.channel` | Notification log channel name, defaults to `discord` |
| `webhook.timeout_sec` | Webhook HTTP timeout in seconds |
| `postgres.dsn` | PostgreSQL DSN |
| `redis.addr` | Redis address |
| `telegram.bot_token` | Telegram Bot token |
| `telegram.default_chat_id` | Default Telegram chat/channel target |
| `api.bearer_token` | Bearer token for protected operator APIs |

支持环境变量覆盖：

```bash
CW_POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/crypto_watchtower?sslmode=disable"
CW_REDIS_ADDR="localhost:6379"
CW_TELEGRAM_BOT_TOKEN="YOUR_BOT_TOKEN"
CW_TELEGRAM_DEFAULT_CHAT_ID="YOUR_CHAT_ID"
CW_API_BEARER_TOKEN="change-me"
CW_BINANCE_ENABLED="true"
CW_OKX_ENABLED="true"
CW_OKX_PUBLIC_WS_BASE_URL="wss://ws.okx.com:8443/ws/v5/public"
CW_OKX_REST_BASE_URL="https://www.okx.com"
CW_OKX_SYMBOLS="BTCUSDT,ETHUSDT,SOLUSDT"
CW_WEBHOOK_ENABLED="false"
CW_WEBHOOK_URL="https://discord.com/api/webhooks/..."
CW_WEBHOOK_CHANNEL="discord"
CW_WEBHOOK_TIMEOUT_SEC="10"
```

Discord / Webhook 通知默认关闭。开启后会复用现有告警链路，并在 `notification_logs` 中按 `channel`、`target`、`status`、`error_message` 记录每个渠道的投递结果。

## Run With Docker Compose

推荐使用 Docker Compose 启动完整本地环境：

```bash
docker compose -f deployments/docker-compose.yml up -d --build
```

该命令会启动：

- `crypto-watchtower-app`
- `crypto-watchtower-postgres`
- `crypto-watchtower-redis`

如果本机 `8080` 已被占用，可以指定宿主机端口：

```bash
APP_HTTP_PORT=18080 docker compose -f deployments/docker-compose.yml up -d --build
```

健康检查：

```bash
curl http://127.0.0.1:8080/health
```

如果使用 `APP_HTTP_PORT=18080`：

```bash
curl http://127.0.0.1:18080/health
```

打开后台：

```text
http://127.0.0.1:8080/admin
```

后台支持中英双语切换；写操作和 Admin 数据加载需要填写 `api.bearer_token` 对应的 Bearer Token。

查看日志：

```bash
docker logs -f crypto-watchtower-app
```

停止服务：

```bash
docker compose -f deployments/docker-compose.yml down
```

保留数据卷只停服务；如果需要清理 PostgreSQL/Redis 数据卷：

```bash
docker compose -f deployments/docker-compose.yml down -v
```

## Build Docker Image

单独构建应用镜像：

```bash
docker build -f deployments/Dockerfile -t crypto-watchtower:test .
```

指定版本 tag：

```bash
docker build -f deployments/Dockerfile -t crypto-watchtower:0.1.0 .
```

## Run App Container Manually

如果 PostgreSQL 和 Redis 已经在外部运行，可以只启动应用容器：

```bash
docker run --rm \
  --name crypto-watchtower-app \
  -p 8080:8080 \
  -e CONFIG_PATH=/app/configs/config.example.yaml \
  -e CW_POSTGRES_DSN="postgres://postgres:postgres@host.docker.internal:5432/crypto_watchtower?sslmode=disable" \
  -e CW_REDIS_ADDR="host.docker.internal:6379" \
  -e CW_TELEGRAM_BOT_TOKEN="YOUR_BOT_TOKEN" \
  -e CW_TELEGRAM_DEFAULT_CHAT_ID="YOUR_CHAT_ID" \
  -e CW_API_BEARER_TOKEN="change-me" \
  crypto-watchtower:test
```

Linux 环境如果无法使用 `host.docker.internal`，请改成宿主机网关 IP 或使用 Docker network。

## Run Locally With Go

启动依赖：

```bash
docker compose -f deployments/docker-compose.yml up -d postgres redis
```

运行服务：

```bash
CONFIG_PATH=configs/config.example.yaml go run ./cmd/server
```

也可以使用脚本：

```bash
./scripts/run-local.sh
```

## Operator APIs

健康检查：

```bash
curl http://127.0.0.1:8080/health
```

查看 symbols：

```bash
curl http://127.0.0.1:8080/api/v1/symbols
```

查看规则：

```bash
curl http://127.0.0.1:8080/api/v1/rules
```

更新单条系统规则，写入数据库后会立即生效到运行中的 Rule Engine：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rules \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"exchange":"binance","symbol":"BTCUSDT","rule_type":"large_trade","threshold":120000,"enabled":true}'
```

60 秒累计成交额规则也支持同样的动态覆盖：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rules \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"exchange":"binance","symbol":"BTCUSDT","rule_type":"large_trade_window","threshold":500000,"window_sec":60,"enabled":true}'
```

测试告警接口：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/alerts/test \
  -H "Authorization: Bearer change-me"
```

测试 Telegram 推送：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/telegram/test \
  -H "Authorization: Bearer change-me"
```

写接口默认需要：

```text
Authorization: Bearer <api.bearer_token>
```

如果 `telegram.enabled=true` 且 `telegram.mode=polling`，Bot 会启用以下命令：

```text
/start   保存当前 chat_id；实时告警当前仍发送到配置的默认 chat/channel
/status  查看服务状态摘要
/rules   查看当前启用规则
/test    回发一条测试告警
```

## Admin APIs

以下接口用于后台管理页，默认都需要：

```text
Authorization: Bearer <api.bearer_token>
```

概览：

```bash
curl http://127.0.0.1:8080/api/v1/admin/overview \
  -H "Authorization: Bearer change-me"
```

趋势摘要：

```bash
curl http://127.0.0.1:8080/api/v1/admin/trends \
  -H "Authorization: Bearer change-me"
```

规则列表：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/rules?limit=20&exchange=okx&symbol=BTCUSDT" \
  -H "Authorization: Bearer change-me"
```

告警列表：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/alerts?limit=20&exchange=okx&symbol=BTCUSDT&rule_type=large_trade" \
  -H "Authorization: Bearer change-me"
```

告警相关事件列表：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/events?limit=20&exchange=okx&symbol=BTCUSDT&event_type=agg_trade" \
  -H "Authorization: Bearer change-me"
```

当前 `market_events` 只保存触发告警链路的相关事件，用于排障和运营观察；暂不保存 Binance 全量成交事件流，避免早期 PostgreSQL 写入压力失控。

通知日志：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/notifications?limit=20&status=sent" \
  -H "Authorization: Bearer change-me"
```

`/admin` 页面支持：

- 查看 app、PostgreSQL、Redis 与 collector 运行状态。
- 使用 `exchange`、`symbol`、`event_type`、`rule_type`、`status`、`limit` 过滤列表。
- 编辑系统规则的 `exchange`、`threshold`、`window_sec`、`enabled`。
- 查看 24 小时告警数量、通知成功 / 失败数量和 symbol 告警分布。

## Database Migration

服务启动时会自动执行：

```text
migrations/*.sql
```

已执行的 migration 会记录到：

```text
schema_migrations
```

如果 migration 失败，服务会启动失败，避免半初始化状态继续运行。

## Test

本机有 Go 1.24 时：

```bash
go test ./...
```

使用 Docker 运行测试：

```bash
docker run --rm \
  -v "$PWD":/app \
  -w /app \
  golang:1.24 \
  go test ./...
```

如果网络不稳定导致 Go module 下载失败，可以先构建镜像，再在镜像内运行测试：

```bash
docker build -f deployments/Dockerfile -t crypto-watchtower:test .
docker run --rm crypto-watchtower:test /usr/local/go/bin/go test ./...
```

Docker Compose smoke test：

```bash
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

该脚本会验证 Compose 配置、启动本地栈、检查 `/health`、读取 `/api/v1/rules` 和 `/api/v1/admin/events`，并确认未携带 Bearer Token 的写接口会返回 `401`。

真实 PostgreSQL/Redis 集成测试需要先启动 Docker Compose 依赖，然后显式打开 `integration` build tag：

```bash
CW_INTEGRATION_TESTS=1 \
CW_POSTGRES_DSN="postgres://postgres:postgres@127.0.0.1:5432/crypto_watchtower?sslmode=disable" \
CW_REDIS_ADDR="127.0.0.1:6379" \
go test -tags=integration ./internal/integration -v
```

24 小时稳定性验证流程见：

```text
docs/ops/24h-stability-check.md
```

## Project Layout

```text
cmd/server/          Application entrypoint
internal/api/        HTTP routes and health checks
internal/collector/  Binance WS/REST collectors and normalizers
internal/config/     YAML config loading and env overrides
internal/eventbus/   In-process event bus
internal/model/      Domain models
internal/notifier/   Telegram formatting and delivery
internal/rule/       Rule engine and alert pipeline
internal/scheduler/  Periodic jobs
internal/storage/    PostgreSQL, Redis, repositories, migrations
migrations/          SQL migrations
deployments/         Dockerfile and Docker Compose
configs/             Example config
scripts/             Local helper scripts
docs/                Product and implementation docs
```

## Notes

- App image does not include PostgreSQL or Redis. They run as separate containers in Docker Compose.
- Real Telegram tokens should be passed through environment variables, not committed to the repository.
- Current Docker image is single-stage `golang:1.24` for simplicity. A smaller runtime image can be introduced later.
