# SIT-08 SaaS 认证与订阅全量 E2E 测试报告

## 基本信息

- 测试日期：2026-08-31 至 2026-09-01
- 测试用例：`docs/sit/case/08-saas-auth-subscription-sit.md`
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 运行版本：Golang 1.24、PostgreSQL 16.14、Redis 7.0.15
- 测试端口：App `18080`、PostgreSQL `15433`、Redis `16380`
- 外部依赖：Binance、OKX、Summary、Webhook 均关闭
- 浏览器验证：Playwright CLI，真实 Chromium 页面交互
- 安全配置：重置密码正向流程测试时临时开启开发态 token 暴露；最终 Compose smoke 已恢复正式配置并确认该开关不存在

## 总体结论

SIT-08 最终通过。

- `08-01` 至 `08-46` 最终全部达到预期，最终 API 矩阵为 `46/46` 通过。
- 是否存在产品 bug：是，共 3 个；均已完成 TDD 修复和真实环境复测。
- 是否存在测试代码 bug：是，共 1 个；已修复 token hash 冲突并增加自动清理，集成测试可重复执行。
- 注册、登录、注销、session、修改密码、找回/重置密码、强密码、邮箱规范化、disabled 用户、套餐限制及 Admin/用户鉴权分离均通过。
- 报告、截图和日志中未保留 session token、reset token 或明文密码。

## 测试轮次

| 轮次 | 结果 | 说明 |
| --- | --- | --- |
| 第 1 轮 | 失败于 `08-03` | profile 缺少规范化邮箱，定位并修复 BUG-01。 |
| 第 2 轮 | 失败于 `08-38` | 可空 `alerts.rule_id` 无法扫描，定位并修复 BUG-02。 |
| 第 3 轮 | 失败于 `08-39` | 可空 `notification_logs.error_message` 导致 profile 500，定位并修复 BUG-03。 |
| 第 4 轮 | `46/46` 通过 | 全量 API、数据库状态、Cookie、套餐限制、鉴权和日志脱敏均通过。 |
| 最终门禁 | 通过 | 单元测试、真实集成、Compose smoke、浏览器流程、语法和 diff 检查通过。 |

## Bug、修复与复测

| 编号 | 类型 | 初测问题与根因 | TDD RED 证据 | 修复 | 修复后复测 |
| --- | --- | --- | --- | --- | --- |
| BUG-01 | 产品 | `08-03` 注册邮箱已规范化，但 `/api/v1/user/profile` 没有 `email` 字段。 | `TestServiceProfileMasksTelegramBinding` 编译失败：`profile.Email undefined`。 | `UserProfile` 增加安全的 `email` 字段，`user.Service.Profile` 从已规范化用户记录填充。 | 定向 API/用户服务测试通过；真实 profile 返回小写、去空格邮箱；最终 `08-03` 通过。 |
| BUG-02 | 产品 | `08-38` 用户告警历史返回 500；`alerts.rule_id` 允许 `NULL`，仓储层扫描目标为 Go `string`。 | 新增真实 PostgreSQL 测试，稳定失败：`cannot scan NULL into *string`。 | 告警列表查询使用 `COALESCE(rule_id, '')`，兼容无规则 ID 的历史告警。 | 定向集成测试通过；25 条关联告警请求 `limit=9999` 最终严格返回 20 条。 |
| BUG-03 | 产品 | `08-39` 有成功通知记录后 profile 返回 500；`error_message` 允许 `NULL`，仓储层扫描目标为 Go `string`。 | 新增真实 PostgreSQL 测试，稳定失败：`cannot scan NULL into *string`。 | 通知日志查询使用 `COALESCE(error_message, '')`。 | 定向集成测试通过；Pro/VIP profile 和套餐限制全量复测通过。 |
| TEST-BUG-01 | 测试 | Auth 与 Telegram 集成测试仅使用纳秒时间戳最后一位生成 token hash，多轮执行会撞唯一索引，且未清理测试用户。 | 最终集成门禁出现 session、binding token 唯一索引冲突。 | 改用完整纳秒时间戳生成 64 位 hash，并通过 `t.Cleanup` 删除测试用户及级联数据。 | 真实 PostgreSQL/Redis 集成测试 8 项全部通过，可连续重复运行。 |

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 08-01 | 通过 | `/dashboard` 返回 200；无 Cookie profile 返回 401，页面显示受控 `Load failed: unauthorized`。 |
| 08-02 | 通过 | 强密码注册返回 `code=0`，设置 HttpOnly `cw_session`，响应不含密码 hash。 |
| 08-03 | 通过 | 修复 BUG-01 后，带空格和大写的邮箱在 profile 中为小写、去空格值。 |
| 08-04 | 通过 | 无效邮箱返回 400，message=`valid email is required`。 |
| 08-05 | 通过 | 规范化后的重复邮箱返回 400，message=`email already registered`。 |
| 08-06 | 通过 | P-01 至 P-08 全部返回 400，未创建用户且未设置有效 session。 |
| 08-07 | 通过 | 注册非法 JSON 返回 400，message=`invalid json body`。 |
| 08-08 | 通过 | GET 注册接口返回 405。 |
| 08-09 | 通过 | Logout 返回 200，清除 `cw_session`，浏览器显示 `Logged out.`。 |
| 08-10 | 通过 | 错误密码返回通用 `invalid email or password`，不设置 session。 |
| 08-11 | 通过 | 不存在邮箱返回相同通用错误，不泄露账号存在性。 |
| 08-12 | 通过 | 大小写和空格邮箱可登录到规范化账号。 |
| 08-13 | 通过 | 正确登录返回 200，并设置新的 HttpOnly session。 |
| 08-14 | 通过 | 登录非法 JSON 返回 400。 |
| 08-15 | 通过 | disabled 用户返回 400，message=`user is disabled`，无新 session。 |
| 08-16 | 通过 | 有效 session profile 返回当前用户。 |
| 08-17 | 通过 | 删除 Cookie 后 profile 返回 401。 |
| 08-18 | 通过 | 篡改 Cookie 后 profile 返回 401。 |
| 08-19 | 通过 | 注销后复用旧 Cookie 返回 401。 |
| 08-20 | 通过 | 两个独立 session 均可登录；注销一个不影响另一个。 |
| 08-21 | 通过 | 将指定 session 过期后 profile 返回 401。 |
| 08-22 | 通过 | 未登录修改密码返回 401。 |
| 08-23 | 通过 | 错误当前密码返回 400，message=`invalid current password`。 |
| 08-24 | 通过 | P-01 至 P-08 在修改密码路径全部被拒绝，每次复核旧密码仍可登录。 |
| 08-25 | 通过 | 强密码修改成功；注销后旧密码失败、新密码成功。 |
| 08-26 | 通过 | 修改密码非法 JSON 返回 400。 |
| 08-27 | 通过 | 已存在用户重置请求返回 `accepted=true`；token 仅在测试进程内使用。 |
| 08-28 | 通过 | 不存在用户同样返回 `accepted=true`，无 token。 |
| 08-29 | 通过 | 无效邮箱返回受控接受，不泄露信息。 |
| 08-30 | 通过 | 正确 token 配弱密码返回 400。 |
| 08-31 | 通过 | 错误 token 返回 400，message=`invalid or expired reset token`。 |
| 08-32 | 通过 | 正确 token 重置成功；旧密码失败，新密码成功。 |
| 08-33 | 通过 | 同一 token 重复使用返回 400。 |
| 08-34 | 通过 | 数据库构造过期 token 后确认请求返回 400。 |
| 08-35 | 通过 | Free profile 为 `max_rules=5`、`alert_history=20`。 |
| 08-36 | 通过 | Free 用户 5 条不同规则均创建成功。 |
| 08-37 | 通过 | Free 第 6 条返回 403，规则数保持 5。 |
| 08-38 | 通过 | 修复 BUG-02 后，25 条关联历史请求 `limit=9999` 返回 20 条。 |
| 08-39 | 通过 | Pro 为 50/100；50 条成功，第 51 条返回 403。 |
| 08-40 | 通过 | VIP 为 200/200；200 条成功，第 201 条返回 403。 |
| 08-41 | 通过 | unknown plan 使用 Free 限制；第 6 条返回 403。 |
| 08-42 | 通过 | 仅 Admin Bearer 访问用户 profile 返回 401。 |
| 08-43 | 通过 | 仅用户 Cookie 访问 Admin API 返回 401。 |
| 08-44 | 通过 | 注册/登录响应不含 password hash、session 字段或 session 原值。 |
| 08-45 | 通过 | Cookie 包含 `HttpOnly`、`SameSite=Lax`、`Path=/`，TTL 为 168 小时。 |
| 08-46 | 通过 | 应用日志未出现本轮明文密码、session token 或 reset token。 |

## 浏览器验证

Playwright CLI 以同一真实页面会话完成以下流程并退出 0：

1. 无登录打开 Dashboard，显示受控 unauthorized 状态。
2. 注册测试账号，页面显示 `Registered.`、`Subscription: free`、Plan Rules=5、Alert History=20。
3. 注销后使用错误密码，页面显示 `Login failed: invalid email or password`。
4. 使用正确密码登录，页面显示 `Logged in.`。
5. 修改为新强密码，页面显示 `Password changed.`。
6. 注销后使用新密码登录，页面再次显示 `Logged in.`。

控制台中的 401 来自用例要求的未登录 Dashboard API，400 来自刻意执行的错误密码登录；另有浏览器自动请求 `/favicon.ico` 的 404，不影响认证、订阅或页面交互，未发现 JavaScript 运行时异常或框架错误覆盖层。

Browser 插件独立复核未能初始化：已安装目录为 `26.825.51511`，运行时仍引用不存在的 `26.818.61809/browser-service.mjs`。因此按测试技能 fallback 使用 Playwright CLI；该问题属于本机插件版本状态，不属于被测应用。

截图证据（不含凭据）：

- `/private/tmp/sit08-registered.png`
- `/private/tmp/sit08-final-login.png`

## 最终验证

| 验证项 | 结果 |
| --- | --- |
| SIT-08 API/数据库/安全矩阵 | 通过，`46/46`。 |
| Playwright 注册/登录/注销/改密流程 | 通过，脚本退出码 0。 |
| `go test ./... -count=1` | 通过，全部包退出码 0。 |
| 真实 PostgreSQL/Redis integration | 通过，8 项全部 PASS。 |
| `node --check internal/api/dashboardui/app.js` | 通过。 |
| Docker Compose smoke | 通过，构建、健康、读 API 和写鉴权检查完成。 |
| `git diff --check` | 通过。 |
| 最终 `/health` | `up`，PostgreSQL 与 Redis 均为 `ok`。 |
| 正式 App 环境 | `CW_AUTH_EXPOSE_RESET_TOKEN_IN_DEV` 不存在。 |

## 清理结果

- 删除本轮 API 与浏览器测试账号、session、reset token、规则、25 条测试告警及 25 条关联通知日志。
- 清理后复核：SIT 命名用户 `0`、`sit08-*` 告警 `0`。
- 浏览器已关闭，临时 Cookie 随测试账号删除失效。
- 保留隔离测试栈运行：App `18080`、PostgreSQL `15433`、Redis `16380`，服务健康。
