# SIT-09 用户级 Telegram 绑定与投递全量 E2E 测试报告

## 基本信息

- 测试日期：2026-09-02
- 测试用例：`docs/sit/case/09-user-telegram-fanout-sit.md`
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 运行版本：Golang 1.24、PostgreSQL 16.14、Redis 7.0.15
- 测试端口：App `18080`、PostgreSQL `15433`、Redis `16380`
- Telegram：临时本地 Bot API mock，覆盖 polling、发送成功、HTTP 500 和真实客户端超时；未发送真实 Telegram 消息
- 浏览器验证：Playwright CLI，真实 Chromium 页面交互

## 总体结论

SIT-09 最终通过。

- `09-01` 至 `09-30` 全部达到预期，最终矩阵为 `30/30` 通过。
- 是否存在产品 bug：是，共 4 个；均已通过 TDD 修复并完成真实环境复测。
- 是否存在测试代码 bug：是，共 1 个；已修复临时驱动对 HTML 响应的解析方式，随后全量执行通过。
- 用户绑定 token、真实 mock `/start` polling、多用户隔离、用户规则 fanout、Admin 查询、失败恢复、超时、非法 chat id 和 disabled 用户均通过。
- 报告、保留截图和应用日志不包含完整 chat id、Bot Token、binding token、session 或密码。

## 测试轮次

| 轮次 | 结果 | 说明 |
| --- | --- | --- |
| RED 定位 | 失败，符合预期 | 配置、网络错误脱敏、chat id 校验和 Admin 通知查询 4 项定向测试稳定复现问题。 |
| 定向修复 | 通过 | 三组单元测试和真实 PostgreSQL Admin 通知测试全部转绿。 |
| E2E 第 1 轮 | 测试驱动失败 | 驱动将 `/dashboard` HTML 当作 JSON 解析；修复驱动，不修改产品行为。 |
| E2E 第 2 轮 | `30/30` 通过 | API、数据库、Redis、mock poller、失败和超时矩阵全部通过。 |
| 浏览器与最终门禁 | 通过 | Dashboard、全量 Go 测试、真实依赖集成、Compose smoke、语法、diff 和安全扫描通过。 |

## Bug、修复与复测

| 编号 | 类型 | 初测问题与根因 | RED 证据 | 修复 | 修复后复测 |
| --- | --- | --- | --- | --- | --- |
| BUG-01 | 产品 | Telegram API 基址固定为官方地址，运行中的 App 无法接入用例要求的 mock sender/poller。 | 配置测试编译失败：Telegram 配置不存在 `APIBaseURL`。 | 新增可选 `telegram.api_base_url` 和 `CW_TELEGRAM_API_BASE_URL`；发送器与 poller 共用，默认仍为官方地址。 | 容器连接本地 mock，绑定、成功、HTTP 500 和超时路径全部通过；恢复默认配置后健康检查通过。 |
| BUG-02 | 产品 | Admin 通知查询只传递 status/limit，忽略 scope/user_id，且直接返回原始 target。 | 真实 PostgreSQL 测试返回未脱敏 target，且无法可靠隔离其他用户和系统记录。 | 仓储查询落实 `scope=user/system`、`user_id`、`status` 组合过滤，并在 Admin 读取路径统一将 target 保留末四位。 | `09-20`、`09-21` 通过；真实集成测试仅返回指定用户记录和 masked target。 |
| BUG-03 | 产品/安全 | HTTP transport 错误直接返回 `url.Error`，错误文本包含带 Bot Token 的请求 URL，可能进入通知日志或应用日志。 | 定向测试确认错误 URL 中出现完整凭据路径。 | 将网络和超时错误转换为有界的 `telegram request failed: network error` 或 `telegram request timed out`。 | HTTP 500、网络错误和 31.555 秒超时复测均未出现 token；后续事件继续成功处理。 |
| BUG-04 | 产品/安全 | 绑定服务只校验非空 chat id，非数字、零值和超长值会进入持久化。 | 三组非法 chat id 测试均出现 `ok=true`。 | 绑定前按 Telegram signed int64 语义校验非零整数，非法值受控返回未绑定。 | 定向测试通过；`09-25` 经真实 poller 提交非法值后 profile 仍为未绑定。 |
| TEST-BUG-01 | 测试 | 临时 E2E 驱动统一按 JSON 解析响应，访问 Dashboard HTML 时失败。 | 第一轮在 HTML 文档开头触发 JSON 解析错误。 | 按响应内容安全解析：JSON 正常解码，HTML 保留文本用于状态码断言。 | 第二轮完整 `30/30` 通过。 |

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 09-01 | 通过 | 用户 A 注册成功，Dashboard 返回 200。 |
| 09-02 | 通过 | 未登录创建 binding token 返回 401。 |
| 09-03 | 通过 | 用户 A 创建 token，API 返回 `code=0`、token 和过期时间。 |
| 09-04 | 通过 | 连续创建两个不同 token，均归属用户 A，不会绑定错误用户。 |
| 09-05 | 通过 | 数据库构造过期 token 后，经真实 mock poller 拒绝，profile 仍未绑定。 |
| 09-06 | 通过 | 错误 token 被受控拒绝，不暴露内部错误且不绑定。 |
| 09-07 | 通过 | 用户 A `/start` 绑定成功，profile 仅显示 `****0001`。 |
| 09-08 | 通过 | 同一 token 再次使用被拒绝，原绑定保持不变。 |
| 09-09 | 通过 | 用户 B 注册并生成独立 token。 |
| 09-10 | 通过 | 用户 B 经 poller 绑定成功。 |
| 09-11 | 通过 | 用户 A profile 只包含 A 的 masked chat，不出现 B 数据。 |
| 09-12 | 通过 | 用户 B profile 只包含 B 的 masked chat。 |
| 09-13 | 通过 | 用户 B 携带伪造的 A `user_id` 仍只能读取 B profile。 |
| 09-14 | 通过 | A chat 使用 B 已消费 token 被受控拒绝，A/B 绑定均未改变。 |
| 09-15 | 通过 | 用户 A 创建 `BTCUSDT large_trade threshold=1000` 个人规则。 |
| 09-16 | 通过 | 用户 B 无个人规则。 |
| 09-17 | 通过 | 匹配事件仅发送到用户 A 的 mock chat，用户 B 无发送。 |
| 09-18 | 通过 | 用户 A 通知日志包含 `telegram/sent`，target 为 `****0001`。 |
| 09-19 | 通过 | 用户 B 通知日志不包含用户 A 的事件或通知。 |
| 09-20 | 通过 | Admin 可按 scope、user_id、status 组合过滤，target 已脱敏。 |
| 09-21 | 通过 | 同一事件命中系统规则和用户规则，分别记录 system 与用户 A 通知。 |
| 09-22 | 通过 | 未绑定用户 C 的规则不发送、不产生用户通知且 pipeline 无异常。 |
| 09-23 | 通过 | HTTP 500 写入有界 failed 日志；切回成功模式后下一事件处理成功。 |
| 09-24 | 通过 | Telegram 客户端超时在 31.555 秒内受控结束，错误脱敏，后续事件成功。 |
| 09-25 | 通过 | 超长/非法 chat id 被拒绝，未落库、未展示危险原值。 |
| 09-26 | 通过 | disabled 用户不发送通知，用户 API 返回 403；恢复账号后服务正常。 |
| 09-27 | 通过 | 绑定前 profile 与 Dashboard 数据为 not bound。 |
| 09-28 | 通过 | 绑定后 Reload 仍只显示 masked chat。 |
| 09-29 | 通过 | Create Token 后刷新 profile 不返回或泄露未使用的历史 token。 |
| 09-30 | 通过 | 发送后 Reload，Notification Logs 展示最近 sent/failed 状态及 masked target。 |

## 浏览器验证

Playwright CLI 使用真实 Dashboard 会话完成以下验证：

1. 未登录页面对四个用户 API 返回预期 401，页面以受控状态展示。
2. 用户 A 登录后显示 `Telegram Binding: ****0001`、个人规则和最近投递状态。
3. 通知列表同时显示 sent、HTTP 500 failed、timeout failed 和恢复后的 sent。
4. Create Token 后在截图前将页面中的一次性 token 替换为 `/start ***`；刷新不会恢复历史 token。

控制台中的四个 401 来自用例要求的未登录 Dashboard 初始加载，另有浏览器自动请求 `/favicon.ico` 的 404；未发现 JavaScript 运行时异常。

Browser 能力本轮未提供，因此按 Build Web Apps 测试 fallback 使用 Playwright CLI。

截图证据：

- `/private/tmp/sit09-dashboard-bound.png`
- `/private/tmp/sit09-dashboard-token-redacted.png`

## 最终验证

| 验证项 | 结果 |
| --- | --- |
| SIT-09 API/数据库/mock poller 矩阵 | 通过，`30/30`。 |
| Playwright Dashboard 绑定、token 和通知展示 | 通过。 |
| `go test ./... -count=1` | 通过，全部包退出码 0。 |
| 真实 PostgreSQL/Redis integration | 通过，全部测试退出码 0。 |
| Dashboard/Admin JavaScript 语法检查 | 通过。 |
| Docker Compose smoke | 通过：配置解析、健康、读 API 和写鉴权检查完成。 |
| `git diff --check` | 通过。 |
| 敏感信息扫描 | 通过：仓库测试证据和 App 日志未命中 Bot Token 或 raw binding token。 |
| 最终 `/health` | `up`，PostgreSQL 与 Redis 均为 `ok`。 |

## 清理与环境恢复

- 已先解绑 A/B，再删除本轮和失败重跑产生的 4 个 SIT 测试账号、7 个 binding token、5 个 session、2 条用户规则、8 条通知、8 条告警及 7 条事件。
- 清理后复核：SIT Telegram 用户 `0`、事件 `0`、告警 `0`、通知 `0`。
- 临时 Telegram mock、E2E 驱动、结果中间文件和浏览器 session 已删除；仅保留两张已脱敏截图。
- App 已恢复 `https://api.telegram.org/bot` 默认基址及占位凭据，临时 mock 已停止。
- App `18080`、PostgreSQL `15433`、Redis `16380` 保持运行且健康。

## 环境说明

- 首次 Compose 构建访问 Harbor 时发生 TLS handshake timeout；本地 SIT 按约定使用已缓存的官方 `golang:1.24`、`postgres:16.14` 和 `redis:7.0.15` 镜像完成验证。
- 该网络问题未影响产品功能结论，也未改变测试、预发布或生产镜像约定。
