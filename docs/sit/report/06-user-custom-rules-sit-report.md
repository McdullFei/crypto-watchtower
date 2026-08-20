# SIT-06 用户自定义规则全量 E2E 测试报告

## 基本信息

- 测试日期：2026-08-20
- 测试用例：`docs/sit/case/06-user-custom-rules-sit.md`
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 运行版本：Golang 1.24、PostgreSQL 16.14、Redis 7.0.15
- 外部依赖：Binance/OKX collector、AI Summary、Webhook 关闭；Telegram 使用 `local-sandbox`
- 测试技能：Build Web Apps `frontend-testing-debugging`、Browser `control-in-app-browser`、Superpowers `systematic-debugging`、`test-driven-development`、`verification-before-completion`
- 浏览器验证：Codex In-app Browser，桌面 `1440x900`、移动端 `390x844`

## 总体结论

SIT-06 最终通过。

- 06-01 至 06-33 均完成验证并达到预期。
- 测试中发现 4 个产品 bug，均完成根因定位、TDD 修复和 E2E 复测。
- Free、Pro、VIP 规则上限分别为 5、50、200，超限请求均返回 403 且数据库计数不增加。
- 用户规则严格按 session 隔离，Bearer Token 不能代替用户 session。
- 系统规则和用户规则可被同一事件同时触发，告警记录互不覆盖。
- 1000 条窗口事件处理后 Redis 仅保留最近 61 个成员，窗口 key 有 TTL；查询 API 保持 200 条硬上限。
- 是否存在 bug：是，共 4 个；全部已修复，最终复测通过。

## Bug 与修复复测

| 编号 | 初测问题与根因 | TDD RED 证据 | 修复 | E2E 复测 |
| --- | --- | --- | --- | --- |
| BUG-01 | `window_sec=-1` 返回 200 并写入数据库。根因是 `ruleWriteRequest.toModel` 只处理零值默认值，没有拒绝负数。 | `TestRulesPostRejectsUnsupportedRuleValues/negative_window` 初次得到 200，期望 400。 | 默认值处理后增加负窗口校验，返回 `window_sec must be greater than 0`。 | 返回 400，测试用户规则行数保持 0。 |
| BUG-02 | `rule_type=unknown_type` 返回 200 并写入无法执行的规则。根因是 API 没有规则类型 allowlist。 | `TestRulesPostRejectsUnsupportedRuleValues/unknown_rule_type` 初次得到 200。 | 仅允许 `large_trade`、`large_trade_window`、`liquidation`、`funding_anomaly`。 | 返回 400、message=`unsupported rule_type`，未写入。 |
| BUG-03 | `exchange=unknown` 返回 200 并写入不会参与 collector 的规则。根因是 API 没有交易所 allowlist。 | `TestRulesPostRejectsUnsupportedRuleValues/unknown_exchange` 初次得到 200。 | 仅允许 `binance`、`okx`。 | 返回 400、message=`unsupported exchange`，未写入。 |
| BUG-04 | 已登录用户被设置为 disabled 后，规则和 Profile 返回 401，授权层无法输出用例要求的 403。根因是 `auth.Service.CurrentUser` 提前把 disabled 用户隐藏为无效 session。 | `TestServiceCurrentUserReturnsDisabledAccount` 得到 `ok=false`；`TestChangePasswordRejectsDisabledSession` 得到 200。 | `CurrentUser` 返回有效 session 对应的真实用户状态；用户 API 和修改密码入口统一通过 `requireDashboardUser` 拒绝 disabled。 | 规则、Profile、修改密码均返回 403、message=`user is disabled`；重新登录返回 400。 |

修复后的定向测试：

```text
TestRulesPostRejectsUnsupportedRuleValues                    PASS
TestServiceCurrentUserReturnsDisabledAccount                 PASS
TestChangePasswordRejectsDisabledSession                     PASS
```

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 06-01 | 通过 | Browser 在 `/dashboard` 使用强密码注册成功，状态显示 `Registered.`，Dashboard 正常加载。 |
| 06-02 | 通过 | 创建 Binance `BTCUSDT large_trade threshold=1000`，Personal Rules 正确展示。 |
| 06-03 | 通过 | 创建 OKX 同交易对规则，列表同时保留 Binance 与 OKX 规则。 |
| 06-04 | 通过 | 创建 `large_trade_window threshold=2000 window=120`，列表展示 `window 120s`。 |
| 06-05 | 通过 | `liquidation` 规则保存并展示。 |
| 06-06 | 通过 | `funding_anomaly threshold=0.08` 保存并展示。 |
| 06-07 | 通过 | disabled 规则保存成功，列表展示 `disabled`；后续匹配事件未生成告警或通知。 |
| 06-08 | 通过 | 空 symbol 返回 400、message=`symbol and rule_type are required`，未写入。 |
| 06-09 | 通过 | Browser 输入小写 `btcusdt`，保存后列表展示 `BTCUSDT`。 |
| 06-10 | 通过 | threshold=0 返回 400、message=`threshold must be greater than 0`。 |
| 06-11 | 通过 | threshold=-1 返回 400，未写入。 |
| 06-12 | 通过 | funding 与金额规则的 threshold=0.0001 均可保存。 |
| 06-13 | 通过 | 省略 `window_sec` 后响应和数据库均为默认 60。 |
| 06-14 | 通过 | `window_sec=1` 保存成功。 |
| 06-15 | 通过 | Browser `min=1` 阻止零/负窗口；API 负窗口修复后返回 400。 |
| 06-16 | 通过 | 非法 rule_type 修复后返回 400，数据库无对应规则。 |
| 06-17 | 通过 | 非法 exchange 修复后返回 400，数据库无对应规则，pipeline 无 panic。 |
| 06-18 | 通过 | Browser/API Logout 成功，状态显示 `Logged out.`，规则/告警/通知列表清空。 |
| 06-19 | 通过 | 用户 B 注册并登录成功。 |
| 06-20 | 通过 | 用户 B API 和 Dashboard 均不包含用户 A 数据，Browser 显示 `No data`。 |
| 06-21 | 通过 | 用户 B 请求 `?user_id=<A>` 仍返回 B 的空列表。 |
| 06-22 | 通过 | 用户 B 创建与 A 相同 symbol/type 的规则成功，A/B 各自数据库记录不覆盖。 |
| 06-23 | 通过 | 无 session POST 用户规则返回 401。 |
| 06-24 | 通过 | 仅携带 `Authorization: Bearer change-me` 仍返回 401。 |
| 06-25 | 通过 | Free 用户前 5 条规则全部返回 200，数据库计数为 5。 |
| 06-26 | 通过 | Free 第 6 条返回 403、message=`subscription rule limit exceeded`，计数仍为 5。 |
| 06-27 | 通过 | disabled 用户规则、Profile、改密均返回 403；重新登录返回 `user is disabled`。 |
| 06-28 | 通过 | Pro 前 50 条、VIP 前 200 条成功；第 51/201 条均返回 403，数据库计数保持 50/200。 |
| 06-29 | 通过 | 匹配事件为用户 A 生成 `large_trade` 和 `large_trade_window` 告警；用户 B 告警/通知均为 0。 |
| 06-30 | 通过 | disabled 规则匹配事件后的告警数和通知数均为 0。 |
| 06-31 | 通过 | 同一事件生成 1 条系统告警和 2 条用户告警，共 3 条；用户告警 `rule_id=user:<id>`，互不覆盖。 |
| 06-32 | 通过 | 用户 A 两条规则独立命中并产生 2 条 `telegram/sent` 通知，用户接口有界返回。 |
| 06-33 | 通过 | 1000 次 replay 全部成功；Redis 最终 `ZCARD=61`、`TTL=59`；1000 条查询样本下 limit=5 返回 5、limit=9999 封顶 200。 |

## Browser 证据

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 页面身份 | 通过 | URL=`/dashboard`，title=`CryptoWatchtower Dashboard`。 |
| 非空与错误覆盖层 | 通过 | DOM 包含 Login、Register、Personal Rules、Alert History、Notification Logs；无框架错误覆盖层。 |
| 控制台 | 通过 | 创建规则、触发后 Dashboard、用户 B 隔离三个阶段的 error/warn 均为空。 |
| 桌面布局 | 通过 | 1440x900 下 `clientWidth=1425`、`scrollWidth=1425`，无横向溢出，6 条规则可见。 |
| 移动布局 | 通过 | 390x844 下 `clientWidth=375`、`scrollWidth=375`，规则卡片正常换行，无横向溢出。 |
| 交互闭环 | 通过 | 注册、创建 6 条规则、注销、登录触发用户、登录隔离用户均观察到对应 UI 状态。 |

临时截图证据：

- `/private/tmp/sit06-dashboard-desktop.png`
- `/private/tmp/sit06-dashboard-mobile.png`
- `/private/tmp/sit06-dashboard-mobile-rules.png`
- `/private/tmp/sit06-dashboard-triggered.png`

## 关键数据证据

- 输入校验初测中，负窗口、unknown type、unknown exchange 均实际写入；修复复测三项均返回 400，测试用户规则计数为 0。
- 用户隔离：A 规则数 1；B 初始规则数 0；B 伪造 A 的 `user_id` 后仍为 0；B 创建同名规则后为 1。
- 配额：Free=`5/403/5`、Pro=`50/403/50`、VIP=`200/403/200`，依次表示成功数、下一条状态、最终数据库计数。
- 触发事件 `sit06-trigger-1787192268213`：数据库存在系统 `large_trade`、用户 `large_trade`、用户 `large_trade_window` 三条告警；三条通知均为 `telegram/sent`，其中两条归属用户 A。
- disabled 事件 `sit06-disabled-1787192268213`：告警 0、通知 0。
- 压力窗口：1000 次事件时间覆盖 1000 秒，60 秒窗口最终仅保留 61 个 Redis 成员，且 key 有正 TTL。
- pipeline 按设计仅持久化产生告警的市场事件；查询分页另使用 1000 条隔离数据库样本验证 5/200 上限。

## 最终验证

| 验证项 | 结果 |
| --- | --- |
| Docker Golang 1.24 `go test ./...` | 通过，所有包退出码 0。 |
| 真实 PostgreSQL/Redis integration | 通过，6 个集成测试全部 PASS。 |
| `node --check internal/api/dashboardui/app.js` | 通过。 |
| Docker Compose smoke | 通过，健康、读 API、Admin 认证和未授权写入检查全部完成。 |
| Build Web Apps Browser QA | 通过，桌面、移动、交互、控制台检查完成。 |
| `git diff --check` | 通过。 |

## 清理结果

- 已删除本轮 13 个 `sit06-*` 测试用户及其 session。
- 已删除 275 条测试用户/系统规则、1002 条测试事件、4 条测试告警和 4 条测试通知。
- 清理后复核：`users=0`、`events=0`、`alerts=0`、`system_rules=0`。
- Redis 窗口与去重 key 已按 TTL 过期。
- 清理后 `/health` 状态为 `up`，PostgreSQL、Redis 均为 `ok`。
- App 已恢复 `deployments/.env.local` 默认本地配置：Binance collector 开启，Summary、Webhook 保持默认关闭，Telegram 恢复占位配置；因本机 8080 端口已被占用，仅保留 `APP_HTTP_PORT=18080` 覆盖。
