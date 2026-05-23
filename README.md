# CryptoWatchtower

CryptoWatchtower 是一个基于 Go 的实时币圈异动监控平台。当前阶段聚焦 **Binance 市场数据采集、异常规则判断、Telegram 告警推送、PostgreSQL/Redis 状态管理**，目标是先跑通稳定的实时监控链路，再逐步扩展 Dashboard、Discord、AI Summary、多交易所和 SaaS 能力。

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

暂未完成：

- Web Dashboard
- 多交易所接入

当前已提供第一版后台管理台骨架：

- `/admin` 轻量后台页面
- 管理 API：概览、规则、告警、事件、通知日志

## Architecture

```text
Binance WS / REST
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

Phase 1 只对接 Binance：

- Spot `aggTrade`
- Futures `aggTrade`
- Futures `forceOrder`
- Futures Funding REST

## Requirements

- Docker Desktop / Docker Engine
- Docker Compose
- Optional: Go 1.24 if running without Docker

## Configuration

默认配置文件：

```text
configs/config.example.yaml
```

关键配置项：

| Key | Description |
| --- | --- |
| `binance.spot_ws_base_url` | Binance Spot WebSocket base URL |
| `binance.futures_ws_base_url` | Binance Futures WebSocket base URL |
| `binance.futures_rest_base_url` | Binance Futures REST base URL |
| `binance.symbols` | Monitored symbols |
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
```

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
/start   绑定当前 chat_id
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

规则列表：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/rules?limit=20&symbol=BTCUSDT" \
  -H "Authorization: Bearer change-me"
```

告警列表：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/alerts?limit=20&symbol=BTCUSDT&rule_type=large_trade" \
  -H "Authorization: Bearer change-me"
```

事件列表：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/events?limit=20&symbol=BTCUSDT&event_type=agg_trade" \
  -H "Authorization: Bearer change-me"
```

通知日志：

```bash
curl "http://127.0.0.1:8080/api/v1/admin/notifications?limit=20&status=sent" \
  -H "Authorization: Bearer change-me"
```

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
