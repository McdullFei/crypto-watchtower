# SIT-11 用户通知日志与 Telegram 解绑全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[11]2026-07-01-user-notification-log-unbind.md`

## 测试目标

验证用户只能查看自己的有界通知日志，并可安全解绑 Telegram。覆盖通知日志列表、limit 边界、target 脱敏、状态筛选、解绑前后 profile、投递偏好保留、未登录/越权、重复解绑、解绑后规则命中行为，以及 Admin/operator 通知边界。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- 用户 A/B 均可登录。
- 用户 A 至少绑定过 Telegram 并产生 sent/failed/disabled 等通知日志。
- 可使用 mock Telegram sender。

## 测试数据

| 名称 | 值 |
| --- | --- |
| 用户 A | `sit-log-a-<timestamp>@example.test` / `Strong1!` |
| 用户 B | `sit-log-b-<timestamp>@example.test` / `Strong1!` |
| 规则 | `binance BTCUSDT large_trade threshold=1000` |
| 通知状态 | `sent`、`failed`、`disabled`、`quiet_hours`、`digested` |

## 通知日志读取

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 11-01 | 用户 A 登录并触发 sent 通知。 | mock sender 成功 | `/api/v1/user/notifications` 返回 status=`sent`。 |
| 11-02 | 触发 failed 通知。 | mock sender 返回错误 | 日志 status=`failed`，error_message 有界。 |
| 11-03 | 关闭 delivery 后触发。 | delivery=false | 日志 status=`disabled`。 |
| 11-04 | 查询通知日志默认 limit。 | `/api/v1/user/notifications` | 返回当前用户日志，数量默认有界。 |
| 11-05 | 查询 limit=1。 | `limit=1` | 只返回 1 条最新日志。 |
| 11-06 | 查询 limit=200。 | `limit=200` | 返回不超过 200 条。 |
| 11-07 | 查询 limit=9999。 | `limit=9999` | 后端限制到 200 或计划内上限，不返回无界数据。 |
| 11-08 | 查询 limit=0/-1/abc。 | 非法 limit | 使用默认 limit 或受控处理；不返回 500。 |
| 11-09 | 检查日志字段。 | 响应 body | 包含 alert_id/channel/status/created_at；target 脱敏；不包含完整 chat id、bot token。 |

## 用户隔离

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 11-10 | 用户 B 登录查询通知。 | B session | 不出现用户 A 日志。 |
| 11-11 | 用户 B 带 `user_id=<A>` 查询。 | `/api/v1/user/notifications?user_id=<A>` | 后端忽略 query，只返回 B 日志。 |
| 11-12 | 未登录查询。 | 删除 cookie | 返回 401。 |
| 11-13 | Bearer 无 cookie 查询。 | `Authorization: Bearer change-me` | 返回 401。 |
| 11-14 | disabled 用户查询。 | 用户状态 disabled | 返回 403 或无法建立有效 session。 |

## Dashboard 通知日志展示

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 11-15 | 用户 A 打开 `/dashboard`。 | 已登录 | Notification Logs 区域展示日志行。 |
| 11-16 | 触发新通知后点击 Reload。 | 新 sent/failed 记录 | 最新状态出现在列表顶部或按实现排序展示。 |
| 11-17 | 日志为空的新用户。 | 用户 C | 显示 `No data`，不报错。 |
| 11-18 | 长 error_message。 | mock 长错误 | 页面不撑破布局；后端错误消息有合理限制或页面可承载。 |

## Telegram 解绑

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 11-19 | 绑定前 profile。 | 用户 A 未绑定或解绑状态 | `telegram_bound=false`，chat mask 为空。 |
| 11-20 | 绑定 Telegram。 | `/start <token>` | `telegram_bound=true`，chat mask 非空。 |
| 11-21 | 关闭 delivery。 | PUT delivery false | profile delivery=false。 |
| 11-22 | 点击 Unbind。 | Dashboard `telegram-unbind-button` | 状态 `Telegram unbound.`；profile `telegram_bound=false`；chat mask 为空。 |
| 11-23 | 检查投递偏好保留。 | 解绑后 profile | `telegram_delivery_enabled` 保持解绑前值，不被强制重置。 |
| 11-24 | 重复解绑。 | 再次 DELETE `/api/v1/user/telegram/binding` | 返回成功或受控无害结果；profile 仍未绑定。 |
| 11-25 | 未登录解绑。 | 无 cookie | 返回 401。 |
| 11-26 | 带 `user_id=<B>` 解绑。 | 用户 A session | 只解绑用户 A，不影响用户 B。 |

## 解绑后投递行为

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 11-27 | 解绑后触发用户规则。 | 匹配事件 | 不调用 Telegram sender；不 panic；按实现无日志或记录 skipped/unbound。 |
| 11-28 | 重新绑定后触发。 | 新 token + chat | 恢复发送或受控失败；日志归属用户 A。 |
| 11-29 | 解绑用户通知与 operator 通知。 | 同一事件命中系统规则 | 用户未绑定不影响 operator/admin 通知。 |
| 11-30 | Admin 查询用户通知。 | Bearer + user_id/status | 可观测解绑前后的日志；完整 chat id 不暴露。 |

## 通过标准

- 用户通知日志按 session 隔离且 limit 有界。
- target 和错误信息安全展示。
- Telegram 解绑幂等、安全，不影响其他用户。
- 解绑保留投递偏好，不影响 operator/admin 通知。
- 未登录、Bearer 替代 session、query 越权均失败或被忽略。

## 证据留存

- 用户 A/B notifications API 响应。
- Dashboard Notification Logs 截图。
- 解绑前后 profile 响应。
- mock sender 调用记录，证明解绑后不发送。

## 清理事项

- 解绑测试用户 Telegram。
- 恢复 delivery 默认状态。
