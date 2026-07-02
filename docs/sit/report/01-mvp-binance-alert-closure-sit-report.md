# SIT-01 MVP Binance 告警闭环全量 E2E 测试报告

## 基本信息

- 测试日期：2026-07-02
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 测试用例：`docs/sit/case/01-mvp-binance-alert-closure-sit.md`
- Admin Token：`change-me`
- 测试技能：Build Web Apps frontend-testing-debugging fallback、superpowers:test-driven-development

## 总体结论

SIT-01 最终通过。

- 初测发现 3 个影响用例全绿的问题，均已修复并复测通过。
- 事件、告警、通知闭环不再依赖公网 Binance 实时行情随机到达，已通过 Admin 受保护 replay 入口按用例要求完成“等待或回放”路径。
- Telegram 失败路径使用占位 token 验证 `failed` 日志；成功路径使用本地 `local-sandbox` token 验证 `sent` 日志，不提交真实 Telegram 凭据。

## Bug 与修复复测

| 编号 | 问题 | 修复 | 复测结果 |
| --- | --- | --- | --- |
| BUG-01 | Admin 空事件查询返回 `data:null`，不符合空数组预期。 | Admin 列表响应统一用 `nonNilSlice` 将 nil slice 编码为 `[]`。 | `events?symbol=NOPEUSDT` 返回 `data: []`。 |
| BUG-02 | 无可控事件回放入口，实时行情不可达时 01-12 至 01-17 无法闭环。 | 新增受 Bearer 保护的 `POST /api/v1/admin/replay-event`，直接进入现有 alert pipeline。 | 回放 `agg_trade`、`liquidation`、`funding` 后 events/alerts/notifications 均落库。 |
| BUG-03 | 无真实 Telegram 沙箱凭据时无法验证 `sent` 通知路径。 | Telegram notifier 增加本地沙箱 token：`local-sandbox`，仅该 token 短路为发送成功。 | `status=sent` 查询到 4 条本次 replay 通知日志。 |
| BUG-04 | 大单告警消息缺少交易所、规则类型、阈值字段。 | 大单消息补充交易所、交易对、规则、阈值、成交额。 | `large_trade` 告警消息包含 `binance`、`BTCUSDT`、`large_trade`、`1000.00 USDT`、`2500.00 USDT`。 |

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 01-01 | 通过 | `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` 退出码 0。 |
| 01-02 | 通过 | `/health` HTTP 200；PostgreSQL、Redis 为 `ok`；collector 包含 Binance。 |
| 01-03 | 通过 | 停止 Redis 后 `/health` 仍 HTTP 200，`dependencies.redis.status=error`；恢复 Redis 后为 `ok`。 |
| 01-04 | 通过 | `/admin` 无 token 可渲染，显示 unauthorized，页面未崩溃。 |
| 01-05 | 通过 | 输入 `change-me` 后 Admin 数据加载成功。 |
| 01-06 | 通过 | 创建 `binance/BTCUSDT/large_trade`，阈值 `1000`，API 返回 `code=0`。 |
| 01-07 | 通过 | 创建 `large_trade_window`，阈值 `2000`，窗口 `60s`，API 返回 `code=0`。 |
| 01-08 | 通过 | `threshold=0` 返回 HTTP 400。 |
| 01-09 | 通过 | `symbol=""` 返回 HTTP 400。 |
| 01-10 | 通过 | 无 Bearer 写规则返回 HTTP 401。 |
| 01-11 | 通过 | 错误 Bearer 写规则返回 HTTP 401。 |
| 01-12 | 通过 | 使用 `POST /api/v1/admin/replay-event` 回放 Binance market event，进入现有 pipeline。 |
| 01-13 | 通过 | `events?exchange=binance&symbol=BTCUSDT&limit=50` 查询到本次 replay 事件。 |
| 01-14 | 通过 | 回放 `agg_trade/notional=2500` 生成 `large_trade` 告警。 |
| 01-15 | 通过 | 同一回放大单事件触发 `large_trade_window`，窗口累计 `2500 > 2000`。 |
| 01-16 | 通过 | 回放 `liquidation/notional=4000` 生成 `liquidation` 告警。 |
| 01-17 | 通过 | 回放 `funding_rate=0.12` 生成 `funding_anomaly` 告警。 |
| 01-18 | 通过 | 占位 Telegram token 下回放告警，告警落库，通知日志为 `failed`，服务未退出。 |
| 01-19 | 通过 | `local-sandbox` token 下回放告警，通知日志为 `sent`；大单消息包含交易所、交易对、规则、阈值、成交额。 |
| 01-20 | 通过 | `notifications?status=failed&limit=20` 可筛选失败日志，响应不暴露 bot token。 |
| 01-21 | 通过 | Telegram 失败后继续回放第二条 `liquidation` 事件，第二条告警仍落库。 |
| 01-22 | 通过 | `exchange=binance` 过滤生效。 |
| 01-23 | 通过 | `symbol=BTCUSDT` 过滤生效。 |
| 01-24 | 通过 | `rule_type=large_trade` 过滤生效。 |
| 01-25 | 通过 | `limit=1` 返回 1 条；`limit=9999` 与 `limit=-1` 均未导致无界结果。 |
| 01-26 | 通过 | 不存在 symbol 查询返回 `code=0` 和空数组 `data: []`。 |

## 关键复测证据

- failed 路径：`sit01-failed-large-20260702-1-large-trade` 产生 `telegram/default/failed`，错误为 `telegram request failed: 404`。
- failed 后续处理：`sit01-failed-liquidation-20260702-2-liquidation` 已落库，证明失败发送不会影响后续事件。
- sent 路径：`sit01-sent-large-20260702-1`、`sit01-sent-liquidation-20260702-1`、`sit01-sent-funding-20260702-1` 均回放成功。
- 四类告警：`large_trade`、`large_trade_window`、`liquidation`、`funding_anomaly` 均在 Admin alerts 查询中出现。
- sent 通知：本次 replay 产生 4 条 `telegram/default/sent` 通知日志。
- 服务日志：回放后无 panic；日志包含 `migrations ready`、`http server listening addr=0.0.0.0:8080`。

## 执行命令摘要

```bash
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api ./internal/notifier ./internal/rule ./cmd/server -v'
CW_TELEGRAM_BOT_TOKEN=YOUR_BOT_TOKEN CW_TELEGRAM_DEFAULT_CHAT_ID=YOUR_CHAT_ID APP_HTTP_PORT=18080 docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml up -d --build app
POST /api/v1/admin/replay-event  # 占位 token failed 路径
CW_TELEGRAM_BOT_TOKEN=local-sandbox CW_TELEGRAM_DEFAULT_CHAT_ID=sit-chat APP_HTTP_PORT=18080 docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml up -d --build app
POST /api/v1/admin/replay-event  # local-sandbox sent 路径
```

## 清理结果

- 已禁用本次写入的低阈值测试规则：
  - `large_trade`
  - `large_trade_window`
  - `liquidation`
  - `funding_anomaly`
- 已将本地 app 恢复为占位 Telegram 配置：`YOUR_BOT_TOKEN` / `YOUR_CHAT_ID`。
