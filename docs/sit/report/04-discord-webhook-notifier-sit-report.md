# SIT-04 Discord/Webhook 通知全量 E2E 测试报告

## 基本信息

- 测试日期：2026-07-02
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 测试用例：`docs/sit/case/04-discord-webhook-notifier-sit.md`
- Admin Token：`change-me`
- 测试技能：Build Web Apps frontend-testing-debugging、superpowers:test-driven-development
- 浏览器验证：Build Web Apps 浏览器能力不可用时，按技能要求使用 Playwright CLI fallback

## 总体结论

SIT-04 最终通过。

- 04-01 至 04-24 均完成验证并达到预期。
- 测试中发现 2 个敏感信息保护相关 bug，均按 TDD 流程先补失败测试，再修复并复测通过。
- Webhook 成功、错误状态码、超时、连接失败、多渠道隔离、Admin UI、Admin API 边界均已覆盖。
- 最终通知日志中 `secret-token`、`cw_session`、`postgres://`、`bad-token` 敏感词扫描命中 0 次。

## Bug 与修复复测

| 编号 | 问题 | TDD 失败用例 | 修复 | 复测结果 |
| --- | --- | --- | --- | --- |
| BUG-01 | Webhook 通知日志 target 会保存完整 URL，Discord-compatible path token 或 query secret 可能出现在 Admin/API 中。 | `TestBuildNotificationSendersRedactsDiscordWebhookSecret` 初次失败，target 为完整 webhook URL。 | 新增 `safeWebhookLogTarget`，日志 target 去除 query/fragment，并将 `/api/webhooks/{id}/{token}` 记录为 `/api/webhooks/{id}/***`。 | `go test ./cmd/server -run BuildNotificationSenders -v` 通过；运行时 target 为 `http://host.docker.internal:19090/webhook`。 |
| BUG-02 | Webhook timeout / connection refused 的底层错误包含完整 URL，可能泄露 URL 中的 secret。 | `TestWebhookNotifierRedactsTransportErrorURL` 初次失败，错误信息包含 `secret-token`。 | `WebhookNotifier.Send` 对 transport error 返回受控摘要：timeout 为 `webhook request failed: timeout`，其他连接错误为 `webhook request failed`。 | `go test ./internal/notifier -run Webhook -v` 通过；运行时 timeout/refused 日志均无 secret。 |

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 04-01 | 通过 | `CW_WEBHOOK_ENABLED=false` 启动成功；触发告警后 mock server 记录为空，Telegram 日志正常生成。 |
| 04-02 | 通过 | `CW_WEBHOOK_ENABLED=true` 且 URL 为空时配置校验给出明确错误：`webhook.url is required when webhook is enabled`；服务无 panic。 |
| 04-03 | 通过 | Webhook URL 指向本地 mock success endpoint 后 App 启动成功，触发告警产生 webhook sender 发送记录。 |
| 04-04 | 通过 | `CW_WEBHOOK_CHANNEL=discord-sit` 后通知日志 channel 为 `discord-sit`，Telegram 仍独立记录。 |
| 04-05 | 通过 | 本地 mock server 返回 204，并记录 Webhook POST 请求。 |
| 04-06 | 通过 | Admin API 创建 `binance/BTCUSDT/large_trade/threshold=1000` 规则成功。 |
| 04-07 | 通过 | 通过 Admin replay 触发匹配事件，告警写入，Webhook 收到 1 次 POST。 |
| 04-08 | 通过 | Webhook 请求体包含标题、交易所、交易对、规则类型、阈值和成交额；不包含 Bearer token、session、数据库 DSN。 |
| 04-09 | 通过 | `/api/v1/admin/notifications?status=sent&limit=20` 查询到 webhook `sent` 记录，Telegram 也有独立 `sent` 记录。 |
| 04-10 | 通过 | Playwright CLI 打开 `/admin`，输入 token 后筛选 `Sent`，Notifications 表展示 `discord-sit · sent` 与 `telegram · sent`。 |
| 04-11 | 通过 | mock 返回 400 时 Webhook 日志为 `failed`，错误摘要为 `webhook returned status 400`，告警仍持久化。 |
| 04-12 | 通过 | mock 返回 401 时 Webhook 日志为 `failed`，target 去除 query secret，错误信息不暴露 URL 敏感部分。 |
| 04-13 | 通过 | mock 返回 429 时 Webhook 日志为 `failed`，后续不同告警可继续处理。 |
| 04-14 | 通过 | mock 返回 500 时 Webhook 日志为 `failed`，App 保持运行。 |
| 04-15 | 通过 | mock 延迟大于 timeout 后 Webhook 日志为 `failed`，错误为 `webhook request failed: timeout`，pipeline 后续可继续处理。 |
| 04-16 | 通过 | 连接拒绝时 Webhook 日志为 `failed`，错误为 `webhook request failed`，Telegram 记录自己的状态。 |
| 04-17 | 通过 | Telegram 沙箱成功、Webhook 500 时，同一告警对应 Telegram `sent` 与 Webhook `failed` 两条日志。 |
| 04-18 | 通过 | Telegram bad token、Webhook 204 时，同一告警对应 Telegram `failed` 与 Webhook `sent` 两条日志，App 保持运行。 |
| 04-19 | 通过 | Telegram bad token、Webhook 500 时，同一告警对应两条 `failed` 日志，规则和事件继续处理。 |
| 04-20 | 通过 | 连续触发 10 条唯一告警，mock server 记录 10 次 POST，通知日志为 20 条双渠道记录且均为 `sent`。 |
| 04-21 | 通过 | 无 Bearer 查询 `/api/v1/admin/notifications` 返回 HTTP 401。 |
| 04-22 | 通过 | `status=failed` 查询仅返回 `failed` 状态通知。 |
| 04-23 | 通过 | `limit=1` 返回 1 条，`limit=200` 与 `limit=9999` 返回当前有界结果 61 条。 |
| 04-24 | 通过 | `exchange=binance&symbol=BTCUSDT&limit=20` 查询返回匹配记录或空列表，接口响应正常。 |

## 关键证据

- 成功路径请求体示例包含 `🚨 BTCUSDT 大额buy`、`交易所: binance`、`规则: large_trade`、`阈值: 1000.00 USDT`、`成交额: 3000.00 USDT`。
- Webhook 400/401/429/500 均写入 `failed` 日志，错误摘要分别包含对应 HTTP status。
- Timeout 修复后日志为 `webhook request failed: timeout`；connection refused 修复后日志为 `webhook request failed`。
- 最终 `/api/v1/admin/notifications?limit=80` 返回 61 条记录，channel 集合为 `discord-sit`、`telegram`，敏感词扫描命中 0 次。
- 连续 10 条告警的本地 mock 记录为 10 条，通知日志为 20 条，状态均为 `sent`。
- Admin UI 通过 Playwright CLI 验证：页面可渲染，筛选 `Sent` 后展示 webhook 和 Telegram sent 记录，target 不含 secret。

## 执行命令摘要

```bash
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./cmd/server -run BuildNotificationSenders -v'
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/notifier -run Webhook -v'
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
curl -fsS -H 'Authorization: Bearer change-me' 'http://127.0.0.1:18080/api/v1/admin/notifications?limit=80'
```

## 清理结果

- 已禁用本次写入的低阈值测试规则。
- 已执行默认 smoke，App 恢复为默认本地配置。
- 已停止本地 Webhook mock server。
- 已清理 Playwright CLI 临时目录。
