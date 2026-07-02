# SIT-10 用户规则窗口与 Telegram 投递开关全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[10]2026-07-01-user-rule-window-delivery-controls.md`

## 测试目标

验证用户级滚动窗口规则和 Telegram 投递开关端到端可用：窗口累计、窗口过期、enabled/disabled 规则、Telegram delivery enabled/disabled、状态日志、Dashboard 展示、operator/admin 通知不受用户投递开关影响。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- 用户已注册并可绑定 Telegram mock chat。
- 可通过 mock event 精确控制事件时间和 notional。

## 测试数据

| 名称 | 值 |
| --- | --- |
| 用户 | `sit-window-<timestamp>@example.test` / `Strong1!` |
| 交易所 | `binance` |
| 交易对 | `BTCUSDT` |
| 窗口规则 | `large_trade_window`，threshold=`3000`，window_sec=`60` |
| 单笔事件 | notional=`1000` |
| 超阈事件 | notional=`4000` |

## 窗口规则 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 10-01 | 注册登录用户并绑定 Telegram。 | 用户 + mock chat | profile 显示 bound，delivery enabled。 |
| 10-02 | 创建窗口规则。 | `large_trade_window threshold=3000 window=60 enabled=true` | 规则保存并展示。 |
| 10-03 | 发送第一条窗口内事件。 | t0，notional=1000 | 不生成告警；不发送通知。 |
| 10-04 | 发送第二条窗口内事件。 | t0+10s，notional=1000 | 累计 2000，仍不告警。 |
| 10-05 | 发送第三条窗口内事件。 | t0+20s，notional=1000 | 累计达到 3000，按实现生成或达到阈值时生成用户告警；通知发送。 |
| 10-06 | 发送窗口外事件。 | t0+70s，notional=1000 | 旧事件过期，累计重新计算；不因旧数据误告警。 |
| 10-07 | 单笔超阈事件。 | t0+80s，notional=4000 | 生成告警。 |
| 10-08 | OKX 同 symbol 事件。 | exchange=`okx` | 不命中 Binance 规则。 |
| 10-09 | disabled 规则。 | 将规则 enabled=false 后触发 | 不生成告警和通知。 |

## 投递开关 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 10-10 | delivery 默认状态。 | 新用户绑定后 profile | `telegram_delivery_enabled=true`。 |
| 10-11 | 浏览器关闭 delivery。 | 取消勾选 `telegram-delivery-enabled` | 状态 `Telegram delivery updated.`；profile false；绑定仍存在。 |
| 10-12 | delivery 关闭时触发规则。 | 匹配事件 | 不调用 Telegram sender；notification log status=`disabled`；告警仍写入。 |
| 10-13 | Dashboard Reload。 | 用户 session | `Delivery status: disabled · recent disabled` 或等价状态。 |
| 10-14 | 浏览器开启 delivery。 | 勾选 | profile true。 |
| 10-15 | delivery 开启时触发规则。 | 匹配事件 | 调用 Telegram sender；notification log `sent` 或受控 `failed`。 |
| 10-16 | 未登录修改 delivery。 | PUT `/api/v1/user/telegram/delivery` | 返回 401。 |
| 10-17 | 请求缺少 enabled。 | `{}` | 返回 400，message=`enabled is required`。 |
| 10-18 | enabled 类型错误。 | `{"enabled":"false"}` | 返回 400 或受控错误，不改变原状态。 |
| 10-19 | 带 `user_id=<other>` 修改。 | PUT `/delivery?user_id=999` | 后端忽略 query，仅修改当前 session 用户。 |

## 与 Admin/operator 通知隔离

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 10-20 | 创建系统规则。 | Admin Bearer 创建相同阈值规则 | 系统规则保存。 |
| 10-21 | 用户 delivery 关闭时触发事件。 | 同一事件命中系统和用户规则 | 用户通知 status=`disabled`；operator/admin Telegram/Webhook 仍按系统配置发送或记录。 |
| 10-22 | 查询 Admin notifications。 | Bearer + status 过滤 | 能看到系统通知和用户 disabled 记录，scope/user_id 不混淆。 |
| 10-23 | 用户查询 notifications。 | 用户 session | 只看到自己的用户通知日志，不看到 operator 默认通知。 |

## 边界与稳定性

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 10-24 | window_sec=1。 | 1 秒窗口 | 只累计极短窗口内事件，过期准确。 |
| 10-25 | window_sec 很大。 | 86400 或项目允许上限 | 若允许则内存有界；若不允许则受控拒绝。 |
| 10-26 | 大量窗口事件。 | 1000 条连续事件 | 不出现 OOM；查询仍按 limit；窗口状态可释放。 |
| 10-27 | 多用户同窗口规则。 | 用户 A/B 相同规则 | 各自窗口和通知隔离。 |
| 10-28 | Telegram sender 超时且 delivery enabled。 | mock timeout | status=`failed`；不会阻塞窗口计算。 |

## 通过标准

- 滚动窗口能正确累计和过期。
- 投递关闭只影响用户 Telegram fanout，不影响告警持久化和 operator/admin 通知。
- delivery API 必须 session 鉴权并忽略外部 user_id。
- disabled、failed、sent 状态均可在用户通知日志中观察。

## 证据留存

- Dashboard delivery 开关截图。
- 窗口事件输入和告警输出对照表。
- 用户 notifications JSON。
- Admin notifications JSON。
- mock Telegram sender 调用记录。

## 清理事项

- 将 delivery 恢复 enabled。
- 删除测试窗口规则。
