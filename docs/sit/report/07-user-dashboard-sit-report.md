# SIT-07 用户 Dashboard 全量 E2E 测试报告

## 基本信息

- 测试日期：2026-08-29
- 测试用例：`docs/sit/case/07-user-dashboard-sit.md`
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 运行版本：Golang 1.24、PostgreSQL 16.14、Redis 7.0.15
- 测试端口：App `18080`、PostgreSQL `15433`、Redis `16380`
- 外部依赖：Binance/OKX collector、AI Summary、Webhook 关闭；Telegram 使用 `local-sandbox`
- 浏览器验证：Build Web Apps Browser，Chrome，桌面 `1440x900`、移动端 `390x844`
- Browser 说明：初始 Chrome 标签页受 1Password 扩展弹层阻断；扩展移除并创建新标签页后 Browser 恢复，最终未使用 Playwright 回退。

## 总体结论

SIT-07 最终通过。

- `07-01` 至 `07-43` 全部达到预期。
- 测试中发现 3 个产品 bug，均完成根因定位、TDD 修复和真实环境复测。
- 是否存在 bug：是，共 3 个；全部已修复，最终复测通过。
- 注册、登录、注销、改密、Telegram、偏好、个人规则、告警历史、通知日志、错误态和响应式流程均可用。
- 页面未暴露 session token、明文密码或完整 Telegram chat id；修复后 500 响应也不再暴露内部数据库连接信息。

## Bug 与修复复测

| 编号 | 初测问题与根因 | TDD RED 证据 | 修复 | 复测结果 |
| --- | --- | --- | --- | --- |
| BUG-01 | 合法时区 `Asia/Shanghai` 返回 400。根因是生产镜像基于 `scratch`，没有系统 IANA zoneinfo。 | 将现有 `TestTelegramNotificationPreferencesUsesSessionUser` 编译为 Linux 测试二进制并在生产 scratch 镜像中运行，得到 400：`telegram_quiet_hours_timezone is invalid`。 | 在用户 API 二进制中嵌入 Go 官方 `time/tzdata`。 | 同一定向测试在 scratch 镜像中 PASS；Browser 保存 `Asia/Shanghai` 成功，Reload 后保持。 |
| BUG-02 | 非严格时间 `7:00` 返回 200 并持久化。根因是 `time.Parse("15:04")` 接受单数字小时，没有强制五字符 `HH:MM`。 | 新增 `TestTelegramNotificationPreferencesRejectsNonPaddedTime`，初次得到 200，期望 400。 | 在时间解析前校验长度为 5 且第三个字符为冒号，再执行时间范围解析。 | 定向测试 PASS；真实 API 对 `7:00` 返回 400，`22:00` 正常。 |
| BUG-03 | PostgreSQL 不可用时 Dashboard 直接显示连接错误原文，包含数据库用户名、库名和内部主机名。根因是 `writeInternalError` 将 `err.Error()` 返回客户端。 | 新增 `TestWriteInternalErrorDoesNotExposeCause`，初次响应包含模拟的数据库连接串。 | 服务端使用结构化日志保留原错误，客户端统一返回 `internal server error`。 | 定向测试 PASS；再次停止 PostgreSQL 后页面显示 `Load failed: internal server error`，旧数据全部清空，恢复数据库后重新 `Loaded.`。 |

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 07-01 | 通过 | `/dashboard` 正常渲染，页面清单中的全部 ID 均存在。 |
| 07-02 | 通过 | `/dashboard/app.js`、`/dashboard/styles.css` 均返回 200，无资源 404。 |
| 07-03 | 通过 | 新上下文自动加载后显示 `Load failed: unauthorized`，无登录态数据。 |
| 07-04 | 通过 | 未登录点击 Reload 不跳转，仍显示受控 unauthorized。 |
| 07-05 | 通过 | 未登录 Create Token 显示 `Telegram binding failed: unauthorized`。 |
| 07-06 | 通过 | 未登录提交完整规则显示 `Save failed: unauthorized`。 |
| 07-07 | 通过 | 强密码用户注册成功，状态为 `Registered.`，四类 Dashboard 数据自动加载。 |
| 07-08 | 通过 | `short1!` 因长度不足被后端拒绝。 |
| 07-09 | 通过 | 缺少大写字母被后端拒绝。 |
| 07-10 | 通过 | 缺少小写字母被后端拒绝。 |
| 07-11 | 通过 | 缺少数字被后端拒绝。 |
| 07-12 | 通过 | 缺少特殊字符被后端拒绝。 |
| 07-13 | 通过 | 包含空白字符被后端拒绝。 |
| 07-14 | 通过 | 注销后错误密码显示 `Login failed: invalid email or password`。 |
| 07-15 | 通过 | 正确密码登录成功，显示 Free 订阅和 Telegram 状态。 |
| 07-16 | 通过 | 未登录改密显示 `Password change failed: unauthorized`。 |
| 07-17 | 通过 | 错误当前密码被拒绝，注销后旧密码仍可登录。 |
| 07-18 | 通过 | 六类弱密码作为新密码均被后端拒绝。 |
| 07-19 | 通过 | 强密码修改成功；旧密码登录失败，新密码登录成功。 |
| 07-20 | 通过 | 显示 `Subscription: free`、Plan Rules=5、Alert History=20。 |
| 07-21 | 通过 | Reload 后 profile/rules/alerts/notifications 成功并发加载，状态 `Loaded.`。 |
| 07-22 | 通过 | 新用户三个列表均显示 `No data`，无 JavaScript error。 |
| 07-23 | 通过 | 生成通知后 profile 显示 `recent sent`。 |
| 07-24 | 通过 | Create Token 显示 `/start <token>` 与过期时间；原始 token 未写入报告。 |
| 07-25 | 通过 | Unbind 显示 not bound，delivery enabled 保持不变。 |
| 07-26 | 通过 | 关闭 delivery 后显示 disabled，保存成功。 |
| 07-27 | 通过 | 重新开启 delivery 后显示 enabled。 |
| 07-28 | 通过 | 修复 BUG-01 后 `22:00-08:00 Asia/Shanghai` 保存并在 Reload 后保持。 |
| 07-29 | 通过 | 摘要 60 分钟保存并在 Reload 后保持。 |
| 07-30 | 通过 | `Mars/Phobos` 返回失败，Reload 后原配置保持。 |
| 07-31 | 通过 | `24:00` 与修复后的 `7:00` 均返回 400。 |
| 07-32 | 通过 | 摘要分钟 `4/1441` 返回 400；边界 `5/1440` 返回 200，最后恢复为 60。 |
| 07-33 | 通过 | 创建 Binance `BTCUSDT large_trade threshold=1000 window=60` 并展示。 |
| 07-34 | 通过 | 创建相同 OKX 规则，列表同时展示 Binance 与 OKX。 |
| 07-35 | 通过 | 空白 symbol 返回 `symbol and rule_type are required`，未新增规则。 |
| 07-36 | 通过 | threshold=0 返回 `threshold must be greater than 0`，未新增规则。 |
| 07-37 | 通过 | 回放 `sit07` mock event 后 Alert History 出现用户 `large_trade` 告警。 |
| 07-38 | 通过 | Notification Logs 显示 `telegram/sent`、alert id 和 `****2798` 脱敏 target。 |
| 07-39 | 通过 | notifications API 的 `limit=1/200/9999` 返回 `1/200/200`；页面固定渲染 20 条且不卡顿。 |
| 07-40 | 通过 | `1440x900` 下 `clientWidth=1425`、`scrollWidth=1425`，无横向溢出。 |
| 07-41 | 通过 | `390x844` 下 `clientWidth=375`、`scrollWidth=375`，全部区域可滚动访问。 |
| 07-42 | 通过 | 连续点击 Reload 5 次后最终状态 `Loaded.`，20 条通知稳定，无未处理 Promise error。 |
| 07-43 | 通过 | 停止 PostgreSQL 后显示安全失败消息并清空旧数据；恢复后重新加载成功。 |

## Browser 证据

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 页面身份 | 通过 | URL=`/dashboard`，title=`CryptoWatchtower Dashboard`。 |
| 非空与错误覆盖层 | 通过 | DOM 包含登录、注册、改密、Telegram、Personal Rules、Alert History、Notification Logs；无框架错误覆盖层。 |
| 控制台 | 通过 | 初始、账户流程、告警通知、大列表、连续 Reload 和最终恢复阶段的 error/warn 均为空。 |
| 交互闭环 | 通过 | 注册、登录、注销、改密、token、解绑、delivery、偏好、规则、告警和通知均通过真实页面操作验证。 |
| 桌面布局 | 通过 | `1440x900` 无横向溢出。 |
| 移动布局 | 通过 | `390x844` 无横向溢出，规则与通知区域可见并可滚动访问。 |

截图证据：

- `/private/tmp/sit07-dashboard-desktop.png`
- `/private/tmp/sit07-dashboard-mobile.png`

API JSON 证据：

- `/private/tmp/sit07-notifications-limit-1.json`
- `/private/tmp/sit07-notifications-limit-200.json`
- `/private/tmp/sit07-notifications-limit-9999.json`

## 最终验证

| 验证项 | 结果 |
| --- | --- |
| `go test ./...` | 通过，所有包退出码 0。 |
| Golang 1.24 scratch 时区定向测试 | 通过。 |
| 真实 PostgreSQL/Redis integration | 通过，6 个集成测试全部 PASS。 |
| `node --check internal/api/dashboardui/app.js` | 通过。 |
| Docker Compose smoke | 通过，构建、健康、读 API 和未授权写检查全部完成。 |
| Build Web Apps Browser QA | 通过，页面、交互、控制台、桌面和移动端检查完成。 |
| `git diff --check` | 通过。 |

## 清理结果

- 删除本轮 1 个 `sit-dashboard-*` 测试用户。
- 删除 7 个 session、1 个 Telegram binding token、2 条用户规则、1 条测试事件、1 条测试告警和 206 条测试通知。
- 清理后复核：`users=0`、`events=0`、`alerts=0`、`notifications=0`。
- 清理后 `/health` 为 `up`，PostgreSQL、Redis 均为 `ok`。
- 保留隔离测试栈运行：App `18080`、PostgreSQL `15433`、Redis `16380`；未影响其他项目容器。
