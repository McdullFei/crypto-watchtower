# SIT-04 Discord/Webhook 通知全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[4]2026-06-29-discord-webhook-notifier.md`

## 测试目标

验证可选 Discord-compatible Webhook 通知渠道端到端可用，并且与 Telegram、规则判断、告警持久化和 Admin 查询相互独立。覆盖成功、失败、超时、错误状态码、配置关闭、敏感信息保护。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- 已有可触发的测试规则和测试事件。
- Webhook 使用本地 mock server 或沙箱 Discord-compatible endpoint。
- Telegram 可同时开启或使用占位配置，用于验证多通知渠道互不阻塞。

## 测试数据

| 名称 | 值 |
| --- | --- |
| Admin Token | `change-me` |
| Webhook channel | `discord` 或配置中的 channel |
| Webhook 成功响应 | HTTP 200 或 204 |
| Webhook 失败响应 | HTTP 400、401、429、500 |
| Webhook 超时 | 响应时间大于 `CW_WEBHOOK_TIMEOUT_SEC` |
| 触发规则 | `binance` / `BTCUSDT` / `large_trade` / threshold=`1000` |

## 配置开关

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 04-01 | Webhook 关闭时启动。 | `CW_WEBHOOK_ENABLED=false` | App 启动；触发告警时不产生 webhook 通知日志；Telegram 行为不受影响。 |
| 04-02 | Webhook 开启但 URL 为空。 | `CW_WEBHOOK_ENABLED=true`，URL 为空 | App 启动或配置校验给出明确错误；不得 panic。 |
| 04-03 | Webhook 开启且 URL 指向 mock success endpoint。 | `CW_WEBHOOK_URL=http://127.0.0.1:<port>/webhook` | App 启动；通知 sender 列表包含 webhook。 |
| 04-04 | 自定义 channel。 | `CW_WEBHOOK_CHANNEL=discord-sit` | 通知日志 channel 或 target 能体现配置值，不影响 Telegram。 |

## 成功路径 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 04-05 | 启动 mock server，返回 204。 | mock `/webhook` | mock server 记录请求。 |
| 04-06 | Admin 创建低阈值规则。 | `BTCUSDT large_trade threshold=1000` | 规则保存成功。 |
| 04-07 | 触发匹配告警。 | 真实或 mock market event | 告警写入；Webhook 收到一次 POST。 |
| 04-08 | 检查 Webhook 请求体。 | mock server 请求记录 | 请求体包含标题/交易所/交易对/规则类型/金额或摘要；不包含 Bearer token、session、数据库 DSN。 |
| 04-09 | 查询通知日志。 | `/api/v1/admin/notifications?status=sent&limit=20` | 存在 webhook sent 记录；Telegram 若开启也有独立记录。 |
| 04-10 | 浏览器 `/admin` 筛选 status=Sent。 | token=`change-me`，status=`Sent` | Notifications 表展示 sent 记录。 |

## 失败与超时路径

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 04-11 | mock 返回 400。 | HTTP 400 body=`bad request` | 通知日志 status=`failed`，error_message 有摘要；告警仍持久化。 |
| 04-12 | mock 返回 401。 | HTTP 401 | status=`failed`；错误不暴露 webhook URL 中的敏感部分。 |
| 04-13 | mock 返回 429。 | HTTP 429 | status=`failed`；不无限重试；下一条告警仍可处理。 |
| 04-14 | mock 返回 500。 | HTTP 500 | status=`failed`；App 不崩溃。 |
| 04-15 | mock 超时。 | sleep > timeout | status=`failed` 或受控 timeout；pipeline 不被永久阻塞。 |
| 04-16 | mock server 关闭。 | connection refused | status=`failed`；Telegram 若可用仍继续发送或记录自己的状态。 |

## 多渠道隔离

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 04-17 | Telegram 成功，Webhook 失败。 | Telegram 沙箱可用，Webhook 500 | Telegram log=`sent`，Webhook log=`failed`；告警只有一条。 |
| 04-18 | Telegram 失败，Webhook 成功。 | Telegram 占位 token，Webhook 204 | Telegram log=`failed`，Webhook log=`sent`；App 不退出。 |
| 04-19 | 两者均失败。 | Telegram 占位，Webhook 500 | 两条 failed 日志；规则和事件仍继续处理。 |
| 04-20 | 连续触发 10 条告警。 | 10 条事件 | 每条告警最多产生配置渠道对应数量的通知日志；不会重复爆发无边界通知。 |

## Admin/API 边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 04-21 | 无 Bearer 查询通知。 | `/api/v1/admin/notifications` | 返回 401。 |
| 04-22 | 按 status 过滤失败。 | `status=failed` | 仅返回失败通知。 |
| 04-23 | 按 limit 过滤。 | `limit=1`、`limit=200`、`limit=9999` | 响应有界。 |
| 04-24 | 按 exchange/symbol 过滤关联告警。 | `exchange=binance&symbol=BTCUSDT` | 返回与筛选条件匹配的通知或空列表，不报错。 |

## 通过标准

- Webhook 成功时外部 endpoint 收到格式正确的 POST。
- Webhook 错误、超时、断连均写入 failed 日志且不影响主流程。
- Telegram 与 Webhook 通知结果独立。
- 敏感信息不出现在请求体、日志和 Admin 页面中。

## 证据留存

- mock server 请求/响应记录。
- Admin Notifications 截图。
- `/api/v1/admin/notifications` JSON。
- 服务日志中 webhook failed/sent 片段。

## 清理事项

- 停止 mock server。
- 恢复 Webhook 配置，避免后续用例误发真实通知。
