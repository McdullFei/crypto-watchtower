# SIT-06 用户自定义规则全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[6]2026-06-30-user-custom-rules.md`

## 测试目标

验证用户自定义规则端到端能力：用户规则模型、session 归属、创建/更新、规则类型边界、阈值和窗口校验、订阅限制、用户隔离、与系统规则并存、触发后生成用户告警和用户通知。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- Dashboard 可访问。
- 用户认证能力可用。
- 可用 mock event 或真实行情触发规则。

## 测试数据

| 名称 | 值 |
| --- | --- |
| 用户 A | `sit-rule-a-<timestamp>@example.test` / `Strong1!` |
| 用户 B | `sit-rule-b-<timestamp>@example.test` / `Strong1!` |
| Free 规则上限 | `5` |
| Pro 规则上限 | `50` |
| VIP 规则上限 | `200` |
| 测试交易对 | `BTCUSDT`、`ETHUSDT` |
| 测试交易所 | `binance`、`okx` |

## 浏览器创建规则

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 06-01 | 打开 `/dashboard` 注册用户 A。 | 用户 A 邮箱，`Strong1!` | 注册成功，加载 Dashboard。 |
| 06-02 | 创建 Binance 大单规则。 | exchange=`binance`，symbol=`BTCUSDT`，rule_type=`large_trade`，threshold=`1000`，window_sec=`60`，enabled=true | `Personal Rules` 出现 `BTCUSDT · large_trade`。 |
| 06-03 | 创建 OKX 大单规则。 | exchange=`okx`，symbol=`BTCUSDT`，rule_type=`large_trade`，threshold=`1000` | 列表同时存在 Binance 和 OKX 规则。 |
| 06-04 | 创建窗口规则。 | rule_type=`large_trade_window`，window_sec=`120`，threshold=`2000` | 保存成功，列表展示 window 120s。 |
| 06-05 | 创建强平规则。 | rule_type=`liquidation` | 保存成功。 |
| 06-06 | 创建资金费率异常规则。 | rule_type=`funding_anomaly`，threshold=`0.08` | 保存成功。 |
| 06-07 | 保存 disabled 规则。 | enabled=false | 规则保存成功但不会触发告警。 |

## 输入校验边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 06-08 | 空 symbol。 | symbol 空 | 浏览器 required 或 API 400；不写入。 |
| 06-09 | symbol 小写。 | `btcusdt` | 前端转为 `BTCUSDT`；保存后列表大写。 |
| 06-10 | threshold 为 0。 | `0` | API 400，message=`threshold must be greater than 0`。 |
| 06-11 | threshold 为负数。 | `-1` | 浏览器 min 或 API 400 阻止。 |
| 06-12 | threshold 为小数。 | `0.0001` | 对 funding_anomaly 可保存；对金额类按实现允许正数。 |
| 06-13 | window_sec 为空。 | 空值 | 后端默认 `60`。 |
| 06-14 | window_sec 为 1。 | `1` | 保存成功。 |
| 06-15 | window_sec 为 0 或负数。 | `0`、`-1` | 浏览器 min 或后端受控拒绝；不得产生异常窗口。 |
| 06-16 | rule_type 非法。 | API 发送 `unknown_type` | 返回 400 或不触发；不得写入无法处理的规则。 |
| 06-17 | exchange 非法。 | API 发送 `unknown` | 返回 400 或后续查询不参与 collector；不得导致 pipeline panic。 |

## 用户归属与越权

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 06-18 | 用户 A 创建规则后注销。 | 点击 Logout | session 清除。 |
| 06-19 | 注册/登录用户 B。 | 用户 B 邮箱，`Strong1!` | 用户 B Dashboard 加载。 |
| 06-20 | 用户 B 查询规则。 | `/api/v1/user/rules` | 不包含用户 A 规则。 |
| 06-21 | 用户 B 带 `user_id=<A>` 查询规则。 | `/api/v1/user/rules?user_id=<A>` | 后端忽略 query，仍返回用户 B 数据。 |
| 06-22 | 用户 B 创建与用户 A 同 symbol/type 规则。 | `BTCUSDT large_trade` | 保存为用户 B 规则，不覆盖用户 A。 |
| 06-23 | 未登录创建用户规则。 | 删除 cookie 后 POST `/api/v1/user/rules` | 返回 401。 |
| 06-24 | 使用 Bearer 但无 session 创建用户规则。 | `Authorization: Bearer change-me` | 返回 401；用户 API 不接受 Bearer。 |

## 订阅限制

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 06-25 | Free 用户连续创建 5 条规则。 | 不同 symbol/rule_type 组合 | 前 5 条成功。 |
| 06-26 | Free 用户创建第 6 条规则。 | 第 6 条 | 返回 403，message=`subscription rule limit exceeded`；不会新增。 |
| 06-27 | 用户状态 disabled 后创建规则。 | 将测试用户状态置 disabled 或使用预置账号 | 返回 403，message=`user is disabled`；Dashboard 加载也应受限。 |
| 06-28 | Pro/VIP 用户规则上限。 | 使用测试数据或数据库调整 plan | Pro 到 50、VIP 到 200 边界符合后端限制。 |

## 规则触发闭环

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 06-29 | 触发用户 A enabled 规则。 | 匹配事件 | 生成用户 A 可见告警；用户 B 不可见。 |
| 06-30 | 触发 disabled 规则。 | 匹配事件 | 不生成用户告警和通知。 |
| 06-31 | 同时存在系统规则和用户规则。 | 同一事件命中两者 | 系统告警和用户告警按各自 scope 记录；不互相覆盖。 |
| 06-32 | 同一用户多规则命中。 | 一条事件命中多个规则 | 每条规则按设计产生告警；通知日志有界。 |
| 06-33 | 大量事件触发窗口规则。 | 连续 1000 条事件 | 窗口累计内存可控；查询接口仍按 limit 返回。 |

## 通过标准

- 用户可通过浏览器创建所有支持类型规则。
- 输入校验、订阅限制、disabled 用户限制均生效。
- 用户规则严格按 session 隔离。
- 系统规则与用户规则并存且互不覆盖。
- 大量事件下窗口逻辑有边界。

## 证据留存

- Dashboard 规则列表截图。
- 创建成功和失败 API 响应。
- 用户 A/B 隔离查询响应。
- 订阅上限第 6 条失败响应。
- 规则触发后的用户告警和通知日志。

## 清理事项

- 删除或禁用测试用户低阈值规则。
- 注销用户 A/B。
