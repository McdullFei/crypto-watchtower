# 24 小时稳定性验证流程

## 目标

验证 CryptoWatchtower 在 Docker Compose 本地栈中连续运行 24 小时，期间服务不崩溃，`/health` 可用，PostgreSQL/Redis 依赖正常，collector 断线后可恢复，Telegram 发送失败不会导致主进程退出。

## 前置条件

- Docker Desktop / Docker Engine 已启动。
- `APP_HTTP_PORT` 未被占用，建议使用 `18080`。
- `CW_API_BEARER_TOKEN` 与 `configs/config.example.yaml` 中的 `api.bearer_token` 保持一致，默认可使用 `change-me`。
- 如果要验证真实 Telegram 发送，设置真实的 `CW_TELEGRAM_BOT_TOKEN` 和 `CW_TELEGRAM_DEFAULT_CHAT_ID`。

## 启动

```bash
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

脚本通过后，保持容器继续运行。

## 巡检命令

每 5 分钟执行一次：

```bash
curl -fsS http://127.0.0.1:18080/health
```

每 30 分钟查看最近日志：

```bash
docker logs --tail 120 crypto-watchtower-app
```

需要关注：

- `migrations ready` 只应在启动阶段出现。
- collector reconnect 日志不应持续刷屏。
- Telegram send error 不应导致进程退出。
- `/health` 中 PostgreSQL、Redis 状态应为 `ok`。
- collector `last_event_at` 应随市场事件持续更新；如果交易所网络异常，`last_error` 可短暂出现，但服务不应退出。

## 结束检查

24 小时后执行：

```bash
curl -fsS http://127.0.0.1:18080/health
docker ps --filter name=crypto-watchtower
docker logs --tail 200 crypto-watchtower-app
```

通过标准：

- `crypto-watchtower-app` 仍处于运行状态。
- `/health` 返回成功。
- PostgreSQL 与 Redis dependency 状态为 `ok`。
- 没有连续重启或崩溃循环。
- 没有无边界刷屏日志。

## 停止

```bash
docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml down
```

如需清理本地测试数据：

```bash
docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml down -v
```
