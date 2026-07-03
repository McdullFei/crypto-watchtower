# SIT-01 至 SIT-04 复测审计报告

## 基本信息

- 测试日期：2026-07-03
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 测试范围：`docs/sit/case/01-mvp-binance-alert-closure-sit.md` 至 `docs/sit/case/04-discord-webhook-notifier-sit.md`
- 测试技能：Build Web Apps `frontend-testing-debugging`、`superpowers:test-driven-development`
- 浏览器验证：Build Web Apps 浏览器能力不可用，本轮按技能要求使用 Playwright CLI fallback，并结合 HTTP/API 证据复核页面资源与交互结果。
- Go 测试运行方式：Docker 容器 `golang:1.24`

## 总体结论

SIT-01 至 SIT-04 本轮复测最终通过。

- SIT-01：Binance 告警闭环、Admin API 边界、Telegram 失败隔离、敏感信息扫描均通过。
- SIT-02：OKX 单交易所、双交易所、OKX 故障隔离、1000 条批量查询边界均通过。
- SIT-03：用户会话、Admin API 权限边界、PostgreSQL/Redis 故障降级、Admin UI 渲染均通过。
- SIT-04：Webhook 成功、失败、超时、断连、多渠道隔离、批量通知、敏感信息保护均通过。
- 本轮发现 1 个真实运行时 bug，已按 TDD 流程补失败测试、修复并复测通过。

## 旧报告复核

| 用例 | 旧报告结论复核 | 本轮补强点 | 最终结果 |
| --- | --- | --- | --- |
| SIT-01 | 旧结论与核心功能一致。 | 补充 `limit=1/9999` 边界、无 Bearer/错误 Bearer、Telegram 失败隔离、失败日志敏感词扫描。 | 通过 |
| SIT-02 | 旧结论与 OKX 功能一致。 | 补充实际禁用低阈值规则、OKX 故障时 Binance replay、1000 条 DB 批量数据、`limit=9999` 有界结果。 | 通过 |
| SIT-03 | 旧结论整体充分。 | 补充 PostgreSQL 与 Redis 分别故障时的 health/API/static page 行为，以及 Admin UI 筛选渲染证据。 | 通过 |
| SIT-04 | 旧报告覆盖面较完整。 | 复测时发现 sender error 会让 replay API 返回 500，已修复为通知失败只写日志，不中断事件处理。 | 通过 |

## Bug 与修复复测

| 编号 | 所属用例 | 问题 | TDD 失败用例 | 修复 | 修复后复测 |
| --- | --- | --- | --- | --- | --- |
| BUG-01 | SIT-04 | Webhook 返回 500 时，通知失败已写入 `failed` 日志，但错误继续冒泡，使 `/api/v1/admin/replay-event` 返回 500。 | 新增 `TestPipelineDoesNotFailEventWhenSenderFails`，初次运行失败，报 `fake sender error`。 | `Pipeline.HandleEvent` 保留通知失败日志，继续后续渠道，不再把外部 sender error 作为事件处理失败返回；持久化与去重错误仍返回。 | `go test ./internal/rule -run "TestPipeline(ContinuesLoggingAfterSenderFailure|DoesNotFailEventWhenSenderFails|LogsEachNotificationChannel)" -v` 通过；运行时 webhook 500 场景返回成功，日志为 `discord-sit=failed`、`telegram=sent`。 |

## 复测结果

| 用例 | 覆盖项 | 关键证据 | 结果 |
| --- | --- | --- | --- |
| SIT-01 | Binance 规则与告警闭环 | `SIT14AUDIT01USDT` replay 3 条事件，生成 `funding_anomaly`、`large_trade`、`large_trade_window`、`liquidation` 4 类告警。 | 通过 |
| SIT-01 | API 边界 | invalid threshold=400，空 symbol=400，无 Bearer=401，错误 Bearer=401，`limit=1` 返回 1，`limit=9999` 返回 3。 | 通过 |
| SIT-01 | Telegram 失败隔离 | `SIT14AUDIT01FAILAUSDT`、`SIT14AUDIT01FAILBUSDT` 各产生告警；失败日志 3 条；`bad-token` 泄露命中 0。 | 通过 |
| SIT-02 | OKX 单交易所 | `SIT14AUDIT02USDT` OKX replay 3 条事件，4 类告警完整；同 symbol Binance 告警数为 0。 | 通过 |
| SIT-02 | 双交易所与故障隔离 | 双交易所 health 可见 Binance 与 OKX collector；OKX WS 指向拒绝端口时，Binance replay 正常成功。 | 通过 |
| SIT-02 | 批量边界 | 直接写入 1000 条 OKX 事件，`limit=20` 返回 20，`limit=9999` 返回 200，返回结果均为 OKX。 | 通过 |
| SIT-03 | 用户会话与 Phase 3 API | 注册强密码用户后 profile=200，规则列表空数组，binding token 存在，delivery disabled 可见，logout 后 profile=401。 | 通过 |
| SIT-03 | Admin API 权限矩阵 | overview/trends/rules/alerts/events/notifications：无 token=401，错误 token=401，正确 token=200/code 0。 | 通过 |
| SIT-03 | 依赖故障 | PostgreSQL 停止时 health 显示 postgres error、Admin events=500；Redis 停止时 health 显示 redis error，dashboard HTML/JS/CSS 仍可取回。 | 通过 |
| SIT-03 | Admin UI | Playwright CLI 输入 token 后加载后台；筛选 OKX 且 limit=1，规则/告警/事件/通知面板均展示 1 条匹配记录。 | 通过 |
| SIT-04 | Webhook success 与批量 | mock webhook 收到 11 个 POST；单条请求体包含交易所、交易对、规则、阈值、成交额；10 个 bulk symbol 产生 20 条双渠道 sent 日志。 | 通过 |
| SIT-04 | Webhook 500 | 修复后 `SIT14AUDIT04FAIL500BUSDT`：`discord-sit=failed`，错误为 `webhook returned status 500`，`telegram=sent`。 | 通过 |
| SIT-04 | Telegram 失败隔离 | `SIT14AUDIT04BADTGUSDT`：`telegram=failed`，错误为 `telegram request failed: 404`，`discord-sit=sent`。 | 通过 |
| SIT-04 | Timeout 与断连 | Timeout 错误为 `webhook request failed: timeout`；connection refused 错误为 `webhook request failed`；Telegram 均为 sent。 | 通过 |
| SIT-04 | Admin API 与敏感扫描 | 无 Bearer notifications=401，failed filter 仅含 failed，`limit=1` 返回 1，`limit=9999` 返回 200，敏感词命中 0。 | 通过 |

## 最终验证

| 验证项 | 命令或方式 | 结果 |
| --- | --- | --- |
| 修复前失败测试 | `go test ./internal/rule -run TestPipelineDoesNotFailEventWhenSenderFails -v` | 初次失败，复现 sender error 冒泡问题。 |
| 修复后相关回归 | `go test ./internal/rule -run "TestPipeline(ContinuesLoggingAfterSenderFailure|DoesNotFailEventWhenSenderFails|LogsEachNotificationChannel)" -v` | 通过 |
| 相关包回归 | `go test ./internal/rule ./cmd/server ./internal/notifier ./internal/config -v` | 通过 |
| 全量 Go 回归 | `go test ./...` | 通过 |
| 默认 Docker smoke | `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` | 通过 |
| 最终 health | `GET /health` | app up，PostgreSQL ok，Redis ok。 |

## 清理结果

- 已禁用本轮 `SIT14AUDIT*` 低阈值测试规则，共 27 条，启用残留 0 条。
- 已停止本地 webhook mock server。
- 已清理 Playwright CLI 临时目录。
- 已恢复默认本地 Docker smoke 配置。
