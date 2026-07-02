# SIT-09 用户级 Telegram 绑定与投递全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[9]2026-07-01-user-telegram-fanout.md`

## 测试目标

验证用户可绑定 Telegram，用户规则命中后按用户 fanout 投递到对应 Telegram chat，并写入带用户归属、目标脱敏、状态可查询的通知日志。覆盖绑定 token、真实或 mock `/start`、多用户隔离、未绑定、重复绑定、过期 token、投递失败。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- Telegram 使用沙箱 bot 或 mock sender/poller。
- 至少可触发用户规则告警。
- 测试证据不得包含完整 chat id、bot token 或 binding token。

## 测试数据

| 名称 | 值 |
| --- | --- |
| 用户 A | `sit-tg-a-<timestamp>@example.test` / `Strong1!` |
| 用户 B | `sit-tg-b-<timestamp>@example.test` / `Strong1!` |
| 用户 A chat id | `100001` 或 mock |
| 用户 B chat id | `100002` 或 mock |
| 规则 | `binance BTCUSDT large_trade threshold=1000` |

## Binding Token E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 09-01 | 用户 A 注册登录。 | 用户 A | Dashboard 加载成功。 |
| 09-02 | 未登录请求 binding token。 | 删除 cookie，POST `/api/v1/user/telegram/binding-token` | 返回 401。 |
| 09-03 | 用户 A 点击 Create Token。 | 登录状态 | 页面显示 `/start <token>` 和 expires_at；API 返回 `code=0`。 |
| 09-04 | 连续点击 Create Token 两次。 | 登录状态 | 两次均生成可用 token；旧 token 是否失效按实现验证，不得绑定到错误用户。 |
| 09-05 | token 过期后绑定。 | 构造过期 token | bot/poller 返回失败或不绑定；profile 仍显示 not bound。 |
| 09-06 | 使用错误 token 绑定。 | `/start bad-token` | 不绑定；不暴露内部错误。 |
| 09-07 | 使用用户 A token 绑定 chat A。 | `/start <A token>` | 用户 A profile `telegram_bound=true`，masked chat id 非空。 |
| 09-08 | 重复使用同一 token。 | 再次 `/start <A token>` | 不重复绑定或返回已使用；不会创建多条有效绑定。 |

## 多用户绑定隔离

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 09-09 | 用户 B 注册登录并生成 token。 | 用户 B | 用户 B token 独立。 |
| 09-10 | 用户 B 绑定 chat B。 | `/start <B token>` | 用户 B profile 绑定 chat B。 |
| 09-11 | 用户 A 查询 profile。 | A session | 只看到 A 的 masked chat；不出现 B chat。 |
| 09-12 | 用户 B 查询 profile。 | B session | 只看到 B 的 masked chat。 |
| 09-13 | 用户 B 带 `user_id=<A>` 查询 profile。 | `/api/v1/user/profile?user_id=<A>` | 返回 B 自己 profile。 |
| 09-14 | chat A 尝试使用用户 B token。 | `/start <B token>` from chat A | 若允许重新绑定，则只绑定 B 到 chat A；不得影响用户 A；若拒绝重复 chat，也必须受控。 |

## 用户规则 Fanout

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 09-15 | 用户 A 创建个人规则。 | `BTCUSDT large_trade threshold=1000` | 规则归属用户 A。 |
| 09-16 | 用户 B 不创建规则。 | 无 | 用户 B 不应收到用户 A 规则通知。 |
| 09-17 | 触发匹配事件。 | notional > threshold | 用户 A 收到 Telegram 或 mock send；用户 B 不收到。 |
| 09-18 | 查询用户 A 通知日志。 | `/api/v1/user/notifications?limit=20` | 有 channel=`telegram`、status=`sent` 或受控 `failed`、target 脱敏。 |
| 09-19 | 查询用户 B 通知日志。 | B session | 不出现用户 A 的通知。 |
| 09-20 | Admin 查询通知日志。 | Bearer + `scope=user` 或 user_id 过滤 | 可按用户/状态筛选；不暴露完整 chat id。 |
| 09-21 | 同一事件同时命中系统规则和用户规则。 | mock event | operator/admin 默认通知与用户通知分别记录，目标和 scope 不混淆。 |

## 未绑定与失败路径

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 09-22 | 用户未绑定时规则命中。 | 用户 C 有规则无 chat | 不发送 Telegram；不 panic；可按实现选择无通知日志或记录 skipped。 |
| 09-23 | 用户绑定但 Telegram sender 失败。 | mock send error | 用户通知日志 status=`failed`，error_message 有界；后续事件仍可处理。 |
| 09-24 | Telegram API 超时。 | mock timeout | pipeline 不永久阻塞；失败受控。 |
| 09-25 | chat id 很长或非法。 | mock 非法 chat id | 绑定或发送失败受控；不写入危险原始值到页面。 |
| 09-26 | 用户 disabled 后规则命中。 | disabled 用户 | 不发送用户通知；用户 API 返回 403 或 session 不可用。 |

## Dashboard 展示

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 09-27 | 绑定前 Dashboard。 | 用户 A | `Telegram Binding: not bound`。 |
| 09-28 | 绑定后 Reload。 | 用户 A | `Telegram Binding:` 展示 masked 值。 |
| 09-29 | 生成 token 后刷新。 | token 未使用 | token 展示是否保留按设计；不能自动泄露历史 token。 |
| 09-30 | 通知发送后 Reload。 | 用户 A | Notification Logs 列表出现最近状态。 |

## 通过标准

- binding token 只能绑定到对应用户且有过期/使用控制。
- 多用户 Telegram 绑定和通知严格隔离。
- 用户规则命中只 fanout 到绑定且启用的目标用户。
- Telegram 失败、超时、未绑定均为受控行为。
- Dashboard/API 不暴露完整 chat id、bot token、binding token 历史。

## 证据留存

- Create Token 前后 Dashboard 截图，遮盖 token。
- Telegram mock/poller 绑定日志，遮盖完整 chat id。
- 用户 A/B profile 和 notifications 响应。
- Admin notifications 过滤响应。

## 清理事项

- 解绑用户 A/B Telegram。
- 删除低阈值个人规则。
