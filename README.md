# crypto-watchtower

Realtime crypto market anomaly monitoring platform with Telegram alerts, AI summaries, and multi-exchange streaming infrastructure.

基于多交易所实时数据流的币圈异动监控平台，支持 Telegram 告警、AI 市场摘要与实时行情基础设施。

## Quick Start

1. 启动依赖服务：

```bash
docker compose -f deployments/docker-compose.yml up -d
```

2. 检查并调整配置：

```bash
cp configs/config.example.yaml /tmp/crypto-watchtower.yaml
```

至少需要修改：

- `telegram.bot_token`
- `telegram.default_chat_id`
- `api.bearer_token`
- `postgres.dsn`（如果不是本地默认端口）
- `redis.addr`（如果不是本地默认端口）

3. 本地运行服务：

```bash
CONFIG_PATH=/tmp/crypto-watchtower.yaml docker run --rm \
  -e CONFIG_PATH=/tmp/crypto-watchtower.yaml \
  -v "$PWD":/app \
  -w /app \
  golang:1.24 \
  go run ./cmd/server
```

4. 健康检查：

```bash
curl http://localhost:8080/health
```

5. 测试受保护接口：

```bash
curl -X POST http://localhost:8080/api/v1/alerts/test \
  -H "Authorization: Bearer change-me"
```

## Current Phase 1 Skeleton

当前仓库已经包含：

- Go 单体服务启动入口
- YAML 配置加载与环境变量覆盖
- Binance 事件标准化骨架
- Rule Engine 基础规则
- PostgreSQL / Redis 连接封装
- Telegram notifier 最小实现
- 基础运维 API
- SQL migration 初稿
