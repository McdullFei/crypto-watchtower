# SIT-13 Phase 3 用户通知阶段收尾全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[13]2026-07-01-phase3-user-notification-closeout.md`

## 测试目标

对 Phase 3 用户通知能力做最终收口回归：认证、Dashboard、用户规则、Telegram 绑定、投递开关、免打扰、摘要、解绑、通知日志、Admin/operator 边界、文档与自动化验证。该用例要求覆盖所有用户通知状态并确认计划 [9] 到 [13] 没有互相回归。

## 前置条件

- SIT-08 到 SIT-12 已可独立通过，或当前环境具备等价数据。
- 服务运行于 `http://127.0.0.1:18080`。
- PostgreSQL 和 Redis 健康。
- Telegram 使用 mock sender 或沙箱 bot。
- 浏览器上下文干净。

## 最终命令验证

| 编号 | 操作 | 预期结果 |
| --- | --- | --- |
| 13-01 | `node --check internal/api/dashboardui/app.js` | JS 语法检查通过。 |
| 13-02 | `docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'` | Go 全量测试通过。 |
| 13-03 | `CW_INTEGRATION_TESTS=1` 运行真实 PostgreSQL/Redis 目标集成测试。 | auth、binding、delivery preference、notification preference、unbind 仓储测试通过。 |
| 13-04 | `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` | smoke 通过。 |
| 13-05 | `curl -fsS http://127.0.0.1:18080/dashboard` | 返回 Dashboard HTML，包含 Phase 3 所需控件 ID。 |
| 13-06 | `git diff --check` | 无 whitespace 错误。 |

## 浏览器主流程

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 13-07 | 打开 `/dashboard`。 | 无 cookie | 页面渲染，未登录 load failed。 |
| 13-08 | 注册用户 A。 | `sit-closeout-a-<timestamp>@example.test` / `Strong1!` | 注册成功并自动登录。 |
| 13-09 | 创建 Telegram binding token。 | 点击 Create Token | 显示 `/start <token>` 和过期时间。 |
| 13-10 | mock `/start` 绑定。 | 用户 A token + chat A | profile 显示 bound，masked chat id。 |
| 13-11 | 创建用户规则。 | `binance BTCUSDT large_trade threshold=1000` | 规则保存并展示。 |
| 13-12 | 触发直接投递。 | delivery enabled、非免打扰、digest=false | 日志 `sent` 或 mock 失败时 `failed`；不出现 disabled/quiet_hours/digested。 |
| 13-13 | 关闭 delivery。 | Dashboard checkbox=false | profile false。 |
| 13-14 | 触发 delivery disabled。 | 匹配事件 | 日志 `disabled`，不调用 sender。 |
| 13-15 | 开启 delivery。 | checkbox=true | profile true。 |
| 13-16 | 开启免打扰并触发。 | 当前时间位于窗口内 | 日志 `quiet_hours`，不调用 sender。 |
| 13-17 | 关闭免打扰，开启 digest。 | digest=true interval=5 | 偏好保存。 |
| 13-18 | 触发 digest。 | 匹配事件 | 日志 `digested`，不立即发送。 |
| 13-19 | flush 摘要。 | 到期 flush | 发送一条摘要；不重复发送同批数据。 |
| 13-20 | 解绑 Telegram。 | 点击 Unbind | profile unbound，delivery 偏好保留。 |
| 13-21 | 解绑后触发。 | 匹配事件 | 不发送到旧 chat；受控跳过或失败。 |

## 通知状态最终矩阵

| 状态 | 前置条件 | 触发动作 | 预期结果 |
| --- | --- | --- | --- |
| `sent` | 绑定、delivery=true、quiet_hours=false、digest=false、sender 成功 | 用户规则命中 | 用户 Telegram mock 收到消息，日志 `sent`。 |
| `failed` | 同上但 sender 返回错误/超时 | 用户规则命中 | 日志 `failed`，error_message 有界，App 不崩溃。 |
| `disabled` | delivery=false | 用户规则命中 | 不调用 sender，日志 `disabled`。 |
| `quiet_hours` | delivery=true，免打扰命中，digest=false | 用户规则命中 | 不调用 sender，日志 `quiet_hours`。 |
| `digested` | delivery=true，digest=true | 用户规则命中 | 不立即发送，日志 `digested`，后续 flush 摘要。 |
| `unbound/skipped` | 用户未绑定 Telegram | 用户规则命中 | 不发送，不 panic，日志按实现受控。 |

## 反向与边界回归

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 13-22 | 弱密码注册。 | `password1!` 等弱密码 | 全部拒绝。 |
| 13-23 | 错误密码登录。 | `Wrong1!` | 登录失败，不设置 session。 |
| 13-24 | 未登录访问用户 API。 | profile/rules/notifications/preferences | 均返回 401。 |
| 13-25 | Bearer 替代 session。 | 用户 API + `Bearer change-me` | 返回 401。 |
| 13-26 | session 替代 Bearer。 | Admin API + cookie | 返回 401。 |
| 13-27 | query user_id 越权。 | 用户 A 请求 `?user_id=<B>` | 后端只使用 A 的 session 用户。 |
| 13-28 | 非法偏好。 | timezone=`Mars/Phobos`、interval=4/1441 | 返回 400，不覆盖旧配置。 |
| 13-29 | Free 规则上限。 | 创建第 6 条规则 | 返回 403。 |
| 13-30 | 通知日志 limit。 | limit=1/200/9999/abc | 返回有界结果。 |
| 13-31 | 大批量事件。 | 1000 条 mock event | 不 OOM；窗口、摘要、列表均有界。 |

## Admin/operator 边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 13-32 | Admin 正确 Bearer 刷新。 | `change-me` | overview/trends/rules/alerts/events/notifications 加载成功。 |
| 13-33 | Admin notifications 过滤用户状态。 | status=`disabled`、`quiet_hours`、`digested` | 可查询对应状态或在设计范围内展示；不暴露完整 chat id。 |
| 13-34 | operator 系统规则通知。 | 系统规则命中 | 用户 delivery/quiet/digest 不影响 operator 默认通知。 |
| 13-35 | 用户解绑后 operator 通知。 | 同一事件命中系统规则 | operator 通知仍按系统配置执行。 |
| 13-36 | Admin 过滤 exchange/symbol/rule_type。 | Binance/OKX + BTCUSDT | 返回匹配数据，不混入其他 exchange。 |

## Dashboard UI 收口

| 编号 | 操作 | 预期结果 |
| --- | --- | --- |
| 13-37 | 检查控件 ID。 | login/register/password/Telegram binding/delivery/preferences/rules/alerts/notifications 控件均存在。 |
| 13-38 | 桌面截图。 | 无控件重叠；长状态文本不遮挡主流程。 |
| 13-39 | 移动截图。 | 表单可滚动完整访问；按钮文字不溢出。 |
| 13-40 | 连续 Reload。 | 数据最终一致；控制台无未处理错误。 |
| 13-41 | 外部 sender 失败时页面刷新。 | Notification Logs 显示 failed；页面不崩溃。 |

## 文档与计划核对

| 编号 | 检查项 | 预期结果 |
| --- | --- | --- |
| 13-42 | 总体计划。 | Phase 3 用户通知相关条目已打勾或标明完成证据。 |
| 13-43 | 计划 [9]。 | Telegram fanout 完成并有测试证据。 |
| 13-44 | 计划 [10]。 | 投递开关和窗口规则完成并有测试证据。 |
| 13-45 | 计划 [11]。 | 通知日志和解绑完成并有测试证据。 |
| 13-46 | 计划 [12]。 | 免打扰和摘要完成并有测试证据。 |
| 13-47 | 用户手册/README。 | 文档描述与当前 Dashboard/API 行为一致。 |

## 通过标准

- 命令验证、浏览器主流程、状态矩阵、反向边界、Admin/operator 边界全部通过。
- 所有用户通知状态都有至少一条证据。
- 用户数据严格按 session 隔离。
- 外部服务失败不影响核心服务。
- 所有列表、窗口、摘要队列均有边界，符合内存友好要求。

## 证据留存

- 六条最终命令输出。
- Dashboard 主流程截图。
- 通知状态矩阵对应的日志 JSON。
- mock Telegram sender 记录。
- Admin notifications/trends 截图。
- 计划文件打勾截图或 diff。

## 清理事项

- 关闭免打扰和摘要。
- 恢复 delivery enabled。
- 解绑测试 Telegram。
- 删除低阈值测试规则。
- 停止 mock sender/server。
