# SIT-12 用户免打扰与摘要投递全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[12]2026-07-01-user-quiet-hours-digest.md`

## 测试目标

验证用户规则 Telegram 通知支持免打扰时段和有界摘要模式。覆盖偏好表单、时间和时区边界、跨天窗口、摘要 interval 边界、直接投递、disabled、quiet_hours、digested、摘要 flush、队列隔离、解绑、Admin/operator 边界。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- 用户已注册、绑定 Telegram mock chat，delivery enabled。
- 可通过测试 harness 控制当前时间或使用可预测时区窗口。
- 可触发用户规则告警并可调用 scheduler flush 或等待摘要间隔。

## 测试数据

| 名称 | 值 |
| --- | --- |
| 用户 A | `sit-qh-a-<timestamp>@example.test` / `Strong1!` |
| 用户 B | `sit-qh-b-<timestamp>@example.test` / `Strong1!` |
| 规则 | `binance BTCUSDT large_trade threshold=1000` |
| 免打扰跨天窗口 | start=`22:00`，end=`08:00`，timezone=`Asia/Shanghai` |
| 免打扰同日窗口 | start=`12:00`，end=`13:00`，timezone=`UTC` |
| 摘要 interval | `5`、`60`、`1440` |

## 偏好 API 与表单边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 12-01 | 未登录读取偏好。 | GET `/api/v1/user/telegram/preferences` | 返回 401。 |
| 12-02 | 未登录保存偏好。 | PUT 偏好 | 返回 401。 |
| 12-03 | 登录后读取默认偏好。 | 用户 A | quiet_hours=false，默认 start/end/timezone，digest=false，interval 默认 60。 |
| 12-04 | 保存合法跨天免打扰。 | enabled=true，22:00-08:00，Asia/Shanghai | 返回 200；Dashboard Reload 后保持。 |
| 12-05 | 保存合法同日免打扰。 | enabled=true，12:00-13:00，UTC | 返回 200。 |
| 12-06 | 时间为空。 | start/end 为空 | 后端按默认值填充或浏览器默认；返回 profile 有默认值。 |
| 12-07 | start 非法。 | `24:00`、`7:00`、`aa:bb` | 返回 400，message=`telegram_quiet_hours_start must use HH:MM`。 |
| 12-08 | end 非法。 | `24:00`、`8:0`、`aa:bb` | 返回 400。 |
| 12-09 | timezone 非法。 | `Mars/Phobos` | 返回 400，message=`telegram_quiet_hours_timezone is invalid`。 |
| 12-10 | digest interval 边界成功。 | `5`、`1440` | 返回 200。 |
| 12-11 | digest interval 边界失败。 | `4`、`1441`、负数 | 返回 400，message=`between 5 and 1440`。 |
| 12-12 | query user_id 越权保存。 | 用户 A PUT `?user_id=<B>` | 只修改用户 A，不影响用户 B。 |

## 免打扰投递行为

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 12-13 | 非免打扰时段直接投递。 | 当前时间不在窗口内，digest=false | 用户 Telegram sender 被调用；日志 status=`sent` 或受控 `failed`。 |
| 12-14 | 免打扰窗口内触发。 | 当前时间在 22:00-08:00 内 | 不调用 Telegram sender；通知日志 status=`quiet_hours`。 |
| 12-15 | 跨天窗口边界 start。 | 时间刚好 22:00 | 视为进入免打扰，记录 `quiet_hours`。 |
| 12-16 | 跨天窗口边界 end。 | 时间刚好 08:00 或之后 | 视实现边界验证，不应两边都命中；记录预期必须一致。 |
| 12-17 | 同日窗口内触发。 | 12:30 UTC | 记录 `quiet_hours`。 |
| 12-18 | 同日窗口外触发。 | 13:01 UTC | 恢复直接投递。 |
| 12-19 | 免打扰关闭后触发。 | quiet_hours=false | 恢复直接投递。 |
| 12-20 | delivery disabled 优先级。 | delivery=false 且在免打扰内 | 记录 `disabled` 或按设计定义的优先级；必须稳定一致。 |
| 12-21 | 用户未绑定。 | unbind 后触发 | 不发送；不 panic；不误写 quiet_hours 为发送成功。 |

## 摘要模式行为

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 12-22 | 开启 digest。 | digest=true，interval=60 | 偏好保存成功。 |
| 12-23 | digest 模式下触发单条告警。 | 匹配事件 | 不立即发送 Telegram；通知日志 status=`digested`。 |
| 12-24 | digest 模式下触发多条告警。 | 3 条事件 | 队列累计，mock sender 未立即调用。 |
| 12-25 | flush 未到期队列。 | 当前时间小于 interval | 不发送或按设计跳过；队列保留。 |
| 12-26 | flush 到期队列。 | 当前时间 >= interval | 发送一条摘要 Telegram；摘要包含多条告警概览。 |
| 12-27 | flush 后再次 flush。 | 无新告警 | 不重复发送同一批摘要。 |
| 12-28 | digest interval=5。 | 5 分钟 | 到期后可发送，边界成功。 |
| 12-29 | digest interval=1440。 | 24 小时 | 未到期不发送，到期后发送。 |
| 12-30 | digest 关闭后触发。 | digest=false | 恢复直接投递。 |

## 队列隔离与有界性

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 12-31 | 用户 A/B 均开启 digest。 | 两个用户各有规则 | 队列按用户隔离。 |
| 12-32 | 只触发用户 A。 | A 规则事件 | flush 只发送 A 摘要，B 无摘要。 |
| 12-33 | 用户 B 触发后 flush。 | B 规则事件 | B 摘要只包含 B 告警。 |
| 12-34 | 大量告警进入摘要队列。 | 1000 条事件 | 队列有上限或摘要内容有界；不 OOM；flush 后列表查询有界。 |
| 12-35 | 摘要 sender 失败。 | mock send error | 失败被记录；队列是否保留/重试按设计，不能无限循环。 |
| 12-36 | 摘要期间用户解绑。 | digested 后 unbind，再 flush | 不发送到旧 chat；失败或跳过受控。 |

## Dashboard 展示

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 12-37 | 保存免打扰后刷新。 | Dashboard Reload | checkbox、start、end、timezone 与保存值一致。 |
| 12-38 | 保存摘要后刷新。 | Dashboard Reload | digest checkbox 和 interval 与保存值一致。 |
| 12-39 | 免打扰日志刷新。 | 产生 quiet_hours | Notification Logs 显示 `quiet_hours`。 |
| 12-40 | 摘要日志刷新。 | 产生 digested | Notification Logs 显示 `digested`；flush 后显示摘要发送结果。 |

## 通过标准

- 偏好表单和 API 对合法输入保存、非法输入拒绝。
- 免打扰跨天和同日窗口行为稳定。
- 摘要模式不立即发送单条通知，flush 后发送有界摘要。
- disabled、unbound、quiet_hours、digested、sent/failed 状态不混淆。
- 用户队列和通知日志严格隔离。

## 证据留存

- 偏好保存前后 profile JSON。
- quiet_hours/digested/sent/failed 通知日志。
- mock Telegram sender 调用记录。
- Dashboard 偏好表单截图。
- 摘要 flush 前后队列或日志证据。

## 清理事项

- 关闭 digest 和 quiet_hours。
- 恢复 delivery enabled。
- 删除测试规则并解绑 Telegram。
