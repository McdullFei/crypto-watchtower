# SIT-10 用户规则窗口与 Telegram 投递开关全量 E2E 测试报告

## 基本信息

- 测试日期：2026-09-02
- 测试用例：`docs/sit/case/10-user-rule-window-delivery-controls-sit.md`
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 运行版本：Golang 1.24 构建镜像、PostgreSQL 16.14、Redis 7.0.15
- 测试端口：App `18080`、PostgreSQL `15433`、Redis `16380`
- 外部数据源：Binance、OKX、Webhook、Summary 全部关闭
- Telegram：临时本地 Bot API mock；未发送真实 Telegram 消息
- 浏览器：Playwright CLI，真实 Chromium 页面交互

## 总体结论

SIT-10 最终通过。

- `10-01` 至 `10-28` 最终全部达到预期，结果为 `28/28` 通过。
- 是否存在产品 bug：是，共 3 个，均已通过 TDD 修复并完成真实 Redis E2E/integration 复测。
- 是否存在测试驱动/环境问题：是，共 4 类，均已定位并修正临时驱动或测试隔离方式，未用产品代码掩盖测试问题。
- 用户窗口累计、精确过期、规则禁用、delivery 开关、session 鉴权、Admin/operator 隔离、1000 事件压力、多用户隔离和 Telegram 超时恢复均通过。
- 报告、截图和仓库改动不包含明文密码、session、Bot Token、binding token 或完整 Telegram chat id。

## 测试轮次

| 轮次 | 结果 | 说明 |
| --- | --- | --- |
| 基线 | 通过 | 授权本机 socket 后，`go test ./... -count=1` 全部通过；Compose 配置和依赖健康。 |
| E2E 驱动校准 | 测试驱动失败 | 修正 Go 默认 JSON 字段名兼容、每轮唯一 mock chat id 和按目标 chat 统计发送证据。 |
| 窗口首轮 | 失败于 `10-07` | `10-06` 的错误告警被墙钟限流掩盖，`10-07` 的合法新窗口告警也被同一限流压制。 |
| TDD 修复 | 通过 | 两个回归测试先稳定 RED，再修复精确 cutoff 和窗口告警的重复限流，定向及真实 Redis 复测转绿。 |
| 独立代码复核 | 初次发现 1 个 Important | 复核发现 Redis 窗口“裁剪、求和、写入、过期”由多条命令组成，内存路径的前后总额也来自两次加锁，存在并发快照不一致风险。 |
| 并发原子性修复 | 通过 | RED 用例先因内存函数仍只返回单值而编译失败；修复后定向测试、race、真实 Redis 双 goroutine 并发和精确 cutoff integration 全部通过，二次复核无 Critical/Important。 |
| 浏览器与 API | 通过 | 真实 Dashboard 完成关闭、Reload、recent disabled、重新开启；鉴权和 Admin 隔离通过。 |
| 边界与压力 | 通过 | 1 秒、86400 秒、1000 事件和多用户隔离通过。 |
| 超时复测 | 通过 | 唯一 symbol 隔离单用户后，约 31.5 秒受控失败、failed 日志、窗口状态和下一事件恢复均通过。 |
| 最终门禁 | 通过 | 全量 Go、真实 PostgreSQL/Redis integration、Compose smoke、JS 语法、diff、健康和清理检查通过。 |

## 产品 Bug、修复与复测

| 编号 | 初测问题与根因 | TDD RED 证据 | 修复 | 修复后复测 |
| --- | --- | --- | --- | --- |
| BUG-01 | 用户窗口将事件时间恰好等于 `event_time-window_sec` 的记录继续保留，使 `t0+70s` 错误再次达到阈值。 | `TestPipelineUserWindowExpiresExactCutoff` 初次得到 2 条告警，预期仍为 1 条。 | 内存窗口删除 `<= cutoff` 的事件；Redis `ZREMRANGEBYSCORE` 同步删除到 cutoff（含）以保持语义一致。 | 单元测试通过；`10-06` 不再误告警，1 秒窗口 `10-24` 通过。 |
| BUG-02 | `large_trade_window` 已用“阈值从下向上穿越”控制重复告警，却又复用 60 秒墙钟限流，快速 replay 时合法的新窗口穿越被压制。 | `TestAllowUserWindowAlertsDoesNotApplyWallClockRateLimit` 的第二个不同 event id 返回 `allowed=false`。 | 窗口型用户告警保留 event-id 去重，但不再叠加墙钟限流；其他用户规则仍保留原限流。 | 单元测试通过；真实 Redis 下 `10-07` 第二次合法穿越生成告警并发送。 |
| BUG-03 | Redis 用户窗口原先以多条独立命令执行裁剪、读取、写入和过期，并发事件可能读取相同旧快照而漏报/重复穿越；内存路径也分两次加锁读取更新前后总额。 | `TestUpdateMemoryUserWindowReturnsOneLockedSnapshot` 初次编译失败：调用方要求同一次更新返回两个值，而实现仅返回一个值；`TestUpdateRedisUserWindowUsesOneAtomicScript` 要求单次 Eval。 | 内存路径在一个临界区返回 `previous/updated`；Redis 改为单个 Lua Eval 原子执行 `ZREMRANGEBYSCORE`、求和、`ZADD`、`EXPIRE`，并校验返回类型。 | 定向测试与 `go test -race ./internal/rule` 通过；新增真实 Redis integration 以两个 goroutine 验证快照依次为 `0→1000`、`1000→2000`，ZCARD=2、TTL>0，并验证精确 cutoff 后仅保留新事件；独立二次复核无阻断项。 |

## 测试驱动与环境问题

| 编号 | 问题 | 处理与结果 |
| --- | --- | --- |
| TEST-BUG-01 | 临时驱动按 `rule_type` 读取 `AlertRule`，实际响应为 Go 默认字段 `RuleType`。 | 驱动兼容 snake_case/PascalCase；产品接口未修改。 |
| TEST-BUG-02 | 多轮执行复用固定 mock chat id，触发 `idx_users_telegram_chat_id` 唯一约束。 | 每轮使用唯一 chat id；通过 token hash、poller offset 和临时诊断日志确认根因，诊断改动已撤销。 |
| TEST-BUG-03 | `10-12`、`10-15` 用 mock 全局发送数断言，历史测试用户同时匹配导致假失败。 | 改为按当前用户目标 chat 统计；operator 场景只统计默认 operator chat。 |
| TEST-BUG-04 | timeout 首轮使用 BTCUSDT 造成历史用户 fanout，且客户端门槛 25 秒短于 notifier 的 3 次重试总时长。 | 使用唯一 symbol 只命中当前用户，并按 3×10 秒请求加退避设置 30–38 秒门槛；最终通过。 |

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 10-01 | 通过 | 用户注册、session、mock Telegram 绑定和默认 delivery enabled 均正常；profile 仅返回 masked chat。 |
| 10-02 | 通过 | `large_trade_window threshold=3000 window=60 enabled=true` 保存并展示。 |
| 10-03 | 通过 | 第一条 1000 事件只累计，不生成告警或通知。 |
| 10-04 | 通过 | 第二条窗口内事件累计到 2000，仍不告警。 |
| 10-05 | 通过 | 第三条事件累计到 3000，生成用户告警并记录 `sent`。 |
| 10-06 | 通过 | 修复后精确 cutoff 事件过期，`t0+70s` 未因旧数据误告警。 |
| 10-07 | 通过 | 4000 单笔事件形成新的窗口阈值穿越并生成告警。 |
| 10-08 | 通过 | OKX 同 symbol 事件不匹配 Binance 用户规则。 |
| 10-09 | 通过 | 规则 disabled 后不生成用户告警或通知。 |
| 10-10 | 通过 | 新用户 `telegram_delivery_enabled=true`。 |
| 10-11 | 通过 | Playwright 真实点击关闭 delivery，页面显示 `Telegram delivery updated.`，绑定保持。 |
| 10-12 | 通过 | delivery 关闭时告警仍持久化，日志为 `disabled`，当前用户 chat 无发送。 |
| 10-13 | 通过 | Dashboard Reload 显示 `Delivery status: disabled · recent disabled`。 |
| 10-14 | 通过 | Playwright 真实点击重新开启，profile 持久化为 true。 |
| 10-15 | 通过 | delivery 开启时 mock 收到当前 chat 发送，通知日志为 `sent`。 |
| 10-16 | 通过 | 未登录 PUT delivery 返回 401。 |
| 10-17 | 通过 | `{}` 返回 400，message=`enabled is required`。 |
| 10-18 | 通过 | `enabled` 字符串类型返回 400，原状态不变。 |
| 10-19 | 通过 | query 中伪造其他 `user_id` 被忽略，只修改当前 session 用户。 |
| 10-20 | 通过 | Admin Bearer 保存同类系统规则。 |
| 10-21 | 通过 | 用户 delivery 关闭时记录用户 `disabled`；operator 默认 chat 仍发送系统通知。 |
| 10-22 | 通过 | Admin 可按 status/scope/user_id 区分系统与用户通知。 |
| 10-23 | 通过 | 用户 API 只返回自身通知，不返回 operator 默认目标。 |
| 10-24 | 通过 | 1 秒窗口正确淘汰 2 秒前事件。 |
| 10-25 | 通过 | 86400 秒窗口受控接受，Redis 状态具有正 TTL。 |
| 10-26 | 通过 | 1000 条连续事件约 3.981 秒完成；Redis ZCARD=1000、TTL=86459 秒；查询仍按 Free 上限返回不超过 20 条。 |
| 10-27 | 通过 | 用户 A/B 相同规则分别累计并各产生一条独立 `sent` 通知。 |
| 10-28 | 通过 | 三次 Telegram 10 秒超时及退避后记录 `failed`；窗口 ZCARD=1；mock 恢复后下一事件 `sent`。 |

## 浏览器验证

目标流：`/dashboard` → 登录已绑定用户 → 关闭 delivery → 注入匹配事件 → Reload → 验证 disabled/recent disabled → 重新开启。

Browser 插件未提供，因此按 Build Web Apps fallback 使用 Playwright CLI 和真实 Chromium。

| 检查项 | 结果 | 证据 |
| --- | --- | --- |
| Page identity | 通过 | URL 为 `/dashboard`，标题为 `CryptoWatchtower Dashboard`。 |
| 非空页面 | 通过 | Subscription、Telegram Binding、Personal Rules、Alert History 和 Notification Logs 均渲染。 |
| 框架错误覆盖层 | 通过 | DOM 和截图中无错误覆盖层或空白页。 |
| Console health | 通过（已解释） | 4 个 401 来自登录前预期的用户 API，1 个 404 为 favicon；无登录后的 JavaScript 运行时错误。另有浏览器 password-form 可访问性提示，不影响本用例。 |
| Screenshot evidence | 通过 | 已保存 disabled/recent disabled 和重新 enabled 两张脱敏截图。 |
| Interaction proof | 通过 | 两次 checkbox 点击均返回状态文案，profile 与 Reload 后页面状态一致。 |

截图证据：

- `/private/tmp/sit10-dashboard-disabled.png`
- `/private/tmp/sit10-dashboard-enabled.png`

## 压力与内存证据

- 1000 条 replay event：约 `3981 ms`。
- App 内存：`21.59 MiB` → `22.78 MiB`，增加约 `1.19 MiB`。
- 用户窗口 Redis ZSET：`1000` 个成员，TTL `86459` 秒。
- 用户通知查询传入 `limit=9999` 后仍不超过套餐上限 `20`。
- 测试期间未出现 OOM、容器重启、健康失败或无边界查询结果。

## 最终验证

| 验证项 | 结果 |
| --- | --- |
| SIT-10 矩阵 | 通过，`28/28`。 |
| Playwright Dashboard 关闭、Reload、重新开启 | 通过。 |
| 窗口边界、限流例外、内存快照及 Redis 单 Eval 回归 RED→GREEN | 通过。 |
| `go test ./... -count=1` | 通过，全部包退出码 0。 |
| PostgreSQL/Redis integration | 通过，原 9 项存储测试及新增 1 项真实 Redis 原子窗口测试全部 PASS。 |
| `go test -race ./internal/rule -count=1` | 通过。 |
| `node --check internal/api/dashboardui/app.js` | 通过。 |
| Docker Compose smoke | 通过。 |
| `git diff --check` | 通过。 |
| 最终 `/health` | App up，PostgreSQL/Redis 均为 ok。 |

## 清理与恢复

- 恢复当前用户 delivery enabled 后，分轮删除累计 14 个测试账号及关联 session、binding token、用户规则和用户 Redis 状态。
- 删除 `sit10-*` 事件、告警、通知及测试插入的系统规则。
- 清理复核：测试用户 `0`、事件 `0`、告警 `0`、通知 `0`。
- App 恢复默认 Telegram API 基址和占位配置；临时 Telegram mock 已停止。
- App `18080`、PostgreSQL `15433`、Redis `16380` 保持运行。
