# CryptoWatchtower

CryptoWatchtower 是一个基于 Go 的实时币圈异动监控平台。当前阶段聚焦 **Binance / OKX 市场数据采集、异常规则判断、Telegram 告警推送、PostgreSQL/Redis 状态管理、轻量 Dashboard、SaaS 登录会话与本地订阅权益**，目标是先跑通稳定的实时监控链路，再逐步扩展真实支付计费能力。

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

当前已提供第一版后台管理台骨架：

- `/admin` 轻量后台页面
- 管理 API：概览、规则、告警、事件、通知日志
- 后台事件列表当前展示的是“告警相关事件”，不是 Binance 全量市场事件流
- 运营后台已包含运行状态、exchange/symbol/list 过滤、系统规则编辑和 24 小时趋势摘要

当前已提供第一版用户侧 Dashboard：

- `/dashboard` 用户页面，与 `/admin` 运营后台分离。
- 用户 API：注册、登录、退出、密码重置、修改密码、个人规则、个人告警历史、Telegram 绑定状态和 Telegram 投递开关。
- 用户 API 使用 `cw_session` HttpOnly Cookie 解析当前用户，不再要求页面填写 Bearer Token 或显式 `user_id`。
- Telegram `/start <binding_token>` 可绑定用户账号；用户规则含 `large_trade_window` 命中后会向绑定 Telegram chat 投递，并记录带 `user_id` 的通知日志。
- 用户可在 Dashboard 开关 Telegram 投递；关闭后不删除绑定，命中规则会记录 `disabled` 通知日志并跳过发送。
- Free / Pro / VIP 为本地权益层：当前不包含 Stripe、支付宝、微信支付、发票或真实计费。

## Documentation

- [技术方案](docs/币圈异动监控平台技术方案.md)
- [用户手册](docs/用户手册.md)
- [总体开发计划](docs/plan/币圈异动监控平台总体开发计划.md)

## Architecture

```text
Binance WS / REST, OKX Public WS / REST
  -> Collector
  -> EventBus
  -> Rule Engine
  -> Alert Pipeline
  -> Telegram Notifier

PostgreSQL:
  users / user_sessions / password_reset_tokens / alert_rules / market_events / alerts / notification_logs / schema_migrations

Redis:
  dedupe keys / rule rate limit keys / user window-rule state / short-lived state
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
| `summary.enabled` | Enable optional 15-minute AI market summary job; defaults to `false` |
| `summary.interval_sec` | Summary job interval in seconds; defaults to `900` |
| `summary.window_sec` | Summary data window in seconds; defaults to `900` |
| `summary.max_items` | Maximum sampled alerts/events read per summary window; defaults to `50` |
| `summary.provider` | Summary provider, `template` for local deterministic output or `openai_compatible` for HTTP chat completions |
| `summary.disclaimer` | Required disclaimer appended to summaries; defaults to `不构成投资建议` |
| `summary.api_base_url` | OpenAI-compatible API base URL when `summary.provider=openai_compatible` |
| `summary.api_key` | OpenAI-compatible API key; supply through env and never commit real keys |
| `summary.model` | OpenAI-compatible model name |
| `summary.timeout_sec` | OpenAI-compatible request timeout in seconds; defaults to `15` |
| `postgres.dsn` | PostgreSQL DSN |
| `redis.addr` | Redis address |
| `redis.password` | Redis password; local initialization default is `CryptoWatchtower_Local_2026!` |
| `telegram.bot_token` | Telegram Bot token |
| `telegram.default_chat_id` | Default Telegram chat/channel target |
| `api.bearer_token` | Bearer token for protected operator APIs |
| `auth.session_ttl_hours` | Session cookie lifetime in hours; defaults to `168` |
| `auth.password_reset_ttl_min` | Password reset token lifetime in minutes; defaults to `30` |
| `auth.expose_reset_token_in_dev` | Return reset token in non-prod environments for manual testing |

支持环境变量覆盖：

```bash
CW_POSTGRES_DSN="postgres://postgres:CryptoWatchtower_Local_2026!@localhost:5432/crypto_watchtower?sslmode=disable"
CW_REDIS_ADDR="localhost:6379"
CW_REDIS_PASSWORD="CryptoWatchtower_Local_2026!"
CW_TELEGRAM_BOT_TOKEN="YOUR_BOT_TOKEN"
CW_TELEGRAM_DEFAULT_CHAT_ID="YOUR_CHAT_ID"
CW_API_BEARER_TOKEN="change-me"
CW_AUTH_SESSION_TTL_HOURS="168"
CW_AUTH_PASSWORD_RESET_TTL_MIN="30"
CW_AUTH_EXPOSE_RESET_TOKEN_IN_DEV="true"
CW_BINANCE_ENABLED="true"
CW_OKX_ENABLED="true"
CW_OKX_PUBLIC_WS_BASE_URL="wss://ws.okx.com:8443/ws/v5/public"
CW_OKX_REST_BASE_URL="https://www.okx.com"
CW_OKX_SYMBOLS="BTCUSDT,ETHUSDT,SOLUSDT"
CW_WEBHOOK_ENABLED="false"
CW_WEBHOOK_URL="https://discord.com/api/webhooks/..."
CW_WEBHOOK_CHANNEL="discord"
CW_WEBHOOK_TIMEOUT_SEC="10"
CW_SUMMARY_ENABLED="false"
CW_SUMMARY_INTERVAL_SEC="900"
CW_SUMMARY_WINDOW_SEC="900"
CW_SUMMARY_MAX_ITEMS="50"
CW_SUMMARY_PROVIDER="template"
CW_SUMMARY_DISCLAIMER="不构成投资建议"
CW_SUMMARY_API_BASE_URL=""
CW_SUMMARY_API_KEY=""
CW_SUMMARY_MODEL=""
CW_SUMMARY_TIMEOUT_SEC="15"
```

Discord / Webhook 通知默认关闭。开启后会复用现有告警链路，并在 `notification_logs` 中按 `channel`、`target`、`status`、`error_message` 记录每个渠道的投递结果。

AI 市场摘要默认关闭。开启 `CW_SUMMARY_ENABLED=true` 后，服务会每 15 分钟从已落库的 `alerts` 和 `market_events` 中读取有界样本，生成包含 `不构成投资建议` 的摘要并写入 `market_summaries`。本地验证建议使用 `CW_SUMMARY_PROVIDER=template`；生产如使用 `openai_compatible`，请通过 `CW_SUMMARY_API_BASE_URL`、`CW_SUMMARY_API_KEY`、`CW_SUMMARY_MODEL` 注入供应商信息，真实 API Key 禁止提交到仓库。

## Run With Docker Compose

推荐使用 Docker Compose 启动完整本地环境：

```bash
docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml up -d --build
```

本地初始化密码写在 `deployments/.env.local` 和 `configs/config.example.yaml` 中：PostgreSQL 与 Redis 默认使用 `CryptoWatchtower_Local_2026!`。这些是项目初始化密码；生产环境请改用 `deployments/.env.prod` 中的 `CHANGE_ME_*` 占位并替换成真实值。

该命令会启动：

- `crypto-watchtower-app`
- `crypto-watchtower-postgres`
- `crypto-watchtower-redis`

如果本机 `8080` 已被占用，可以指定宿主机端口：

```bash
APP_HTTP_PORT=18080 docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml up -d --build
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
docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml down
```

保留数据卷只停服务；如果需要清理 PostgreSQL/Redis 数据卷：

```bash
docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml down -v
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
  -e CW_POSTGRES_DSN="postgres://postgres:CryptoWatchtower_Local_2026!@host.docker.internal:5432/crypto_watchtower?sslmode=disable" \
  -e CW_REDIS_ADDR="host.docker.internal:6379" \
  -e CW_REDIS_PASSWORD="CryptoWatchtower_Local_2026!" \
  -e CW_TELEGRAM_BOT_TOKEN="YOUR_BOT_TOKEN" \
  -e CW_TELEGRAM_DEFAULT_CHAT_ID="YOUR_CHAT_ID" \
  -e CW_API_BEARER_TOKEN="change-me" \
  crypto-watchtower:test
```

Linux 环境如果无法使用 `host.docker.internal`，请改成宿主机网关 IP 或使用 Docker network。

## Run Locally With Go

启动依赖：

```bash
docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml up -d postgres redis
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

查看某个用户的自定义规则：

```bash
curl "http://127.0.0.1:8080/api/v1/rules?user_id=42"
```

写入某个用户的自定义规则：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rules \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"user_id":42,"exchange":"binance","symbol":"BTCUSDT","rule_type":"large_trade","threshold":120000,"enabled":true}'
```

用户自定义规则会以 `scope=user` 写入 `alert_rules`，当前不会直接覆盖全局实时 Rule Engine。用户侧页面优先使用 `/api/v1/user/*` 接口。

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
/start                  保存当前 chat_id，兼容默认通道调试
/start <binding_token>  将当前 chat_id 绑定到登录用户账号
/status                 查看服务状态摘要
/rules                  查看当前启用规则
/test                   回发一条测试告警
```

## User Dashboard

浏览器打开：

```text
http://127.0.0.1:8080/dashboard
```

页面提供注册、登录、退出、修改密码、个人规则和个人告警历史入口。用户侧 API 使用 `cw_session` HttpOnly Cookie，不再需要在页面填写 Bearer Token 或 `user_id`。

密码强度由后端强校验：至少 8 位，且必须同时包含大写字母、小写字母、数字和特殊字符。该规则适用于注册、密码重置确认和修改密码。

注册并写入 session cookie：

```bash
curl -i -X POST http://127.0.0.1:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Strong1!"}'
```

登录并写入 session cookie：

```bash
curl -i -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Strong1!"}'
```

以下 curl 示例假设已经把返回的 Cookie 保存到 `cookies.txt`：

```bash
curl -c cookies.txt -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Strong1!"}'
```

退出并撤销服务端 session：

```bash
curl -b cookies.txt -X POST http://127.0.0.1:8080/api/v1/auth/logout
```

申请密码重置。本地 `app.env != "prod"` 且 `auth.expose_reset_token_in_dev=true` 时会返回 `reset_token` 方便手工验证；生产只返回泛化受理响应，不暴露账号是否存在：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/auth/password-reset/request \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

确认密码重置：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/auth/password-reset/confirm \
  -H "Content-Type: application/json" \
  -d '{"token":"RESET_TOKEN_FROM_DEV_RESPONSE","new_password":"Better1!"}'
```

登录后修改密码：

```bash
curl -b cookies.txt -X POST http://127.0.0.1:8080/api/v1/user/password \
  -H "Content-Type: application/json" \
  -d '{"current_password":"Strong1!","new_password":"Better1!"}'
```

查看 Telegram 绑定状态、套餐和限额：

```bash
curl -b cookies.txt "http://127.0.0.1:8080/api/v1/user/profile"
```

返回中包含 `telegram_delivery_enabled` 和 `recent_delivery_status`，Dashboard 用它展示投递开关和最近一次投递状态。

生成 Telegram 绑定 token：

```bash
curl -b cookies.txt -X POST http://127.0.0.1:8080/api/v1/user/telegram/binding-token
```

把响应里的 `token` 发送给 Telegram Bot：

```text
/start <token>
```

绑定成功后，用户规则命中的告警会发送到该用户绑定的 Telegram chat，并在 `notification_logs` 中记录 `user_id/channel/target/status/error_message`。禁用用户和未绑定用户不会触发个人 Telegram 投递。

关闭或开启用户侧 Telegram 投递：

```bash
curl -b cookies.txt -X PUT http://127.0.0.1:8080/api/v1/user/telegram/delivery \
  -H "Content-Type: application/json" \
  -d '{"enabled":false}'
```

关闭投递不会删除 Telegram 绑定。关闭期间个人规则命中仍会写入用户告警和通知日志，通知日志状态为 `disabled`，不会向 Telegram 发送消息。

查看个人规则：

```bash
curl -b cookies.txt "http://127.0.0.1:8080/api/v1/user/rules"
```

写入个人规则，后端从 session 推导用户身份：

```bash
curl -b cookies.txt -X POST http://127.0.0.1:8080/api/v1/user/rules \
  -H "Content-Type: application/json" \
  -d '{"exchange":"binance","symbol":"BTCUSDT","rule_type":"large_trade","threshold":120000,"window_sec":60,"enabled":true}'
```

用户侧 `large_trade_window` 使用按用户和规则隔离的窗口状态，不复用系统规则窗口，也不会互相影响。

查看个人告警历史：

```bash
curl -b cookies.txt "http://127.0.0.1:8080/api/v1/user/alerts?limit=20"
```

个人告警历史通过 `notification_logs.user_id` 关联 `alerts.id` 查询，只展示已有用户通知归属的告警；不会从用户 API 暴露全局后台告警列表。

本地订阅权益当前固定为：Free 最多 5 条个人规则、20 条告警历史；Pro 最多 50 条个人规则、100 条告警历史；VIP 最多 200 条个人规则、200 条告警历史。真实支付计费、发票、组织账号和第三方 OAuth 不包含在当前版本。

Operator API 仍由 `api.bearer_token` 保护，用户 session cookie 不替代运营后台 Bearer Token。

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
CW_POSTGRES_DSN="postgres://postgres:CryptoWatchtower_Local_2026!@127.0.0.1:5432/crypto_watchtower?sslmode=disable" \
CW_REDIS_ADDR="127.0.0.1:6379" \
CW_REDIS_PASSWORD="CryptoWatchtower_Local_2026!" \
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
