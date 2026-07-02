# SIT-08 SaaS 认证与订阅全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[8]2026-06-30-saas-auth-subscription.md`

## 测试目标

验证 SaaS 认证与订阅基础能力端到端可用：注册、登录、注销、session cookie、找回/重置密码、修改密码、强密码复杂度、邮箱规范化、重复注册、disabled 用户、Free/Pro/VIP 规则和告警历史限制、Admin 与用户鉴权分离。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- Dashboard 可访问。
- PostgreSQL/Redis 可用。
- 如果验证重置密码 token，可在开发环境开启 `CW_AUTH_EXPOSE_RESET_TOKEN_IN_DEV=true` 或使用测试 harness 读取 token；不要在证据中泄露真实 token。

## 强密码边界矩阵

| 编号 | 密码 | 预期 |
| --- | --- | --- |
| P-01 | `Short1!` | 7 位，拒绝，提示至少 8 位。 |
| P-02 | `password1!` | 缺少大写，拒绝。 |
| P-03 | `PASSWORD1!` | 缺少小写，拒绝。 |
| P-04 | `Password!` | 缺少数字，拒绝。 |
| P-05 | `Password1` | 缺少特殊字符，拒绝。 |
| P-06 | `Pass word1!` | 包含空格，拒绝。 |
| P-07 | `Pass\tword1!` | 包含 tab，拒绝。 |
| P-08 | `Pass\nword1!` | 包含换行，拒绝。 |
| P-09 | `Strong1!` | 满足复杂度，接受。 |
| P-10 | `Better1!` | 满足复杂度，接受。 |

## 注册 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 08-01 | 浏览器打开 `/dashboard`。 | 无 cookie | 页面加载，未登录 API 显示 Load failed。 |
| 08-02 | 注册强密码账号。 | `sit-auth-<timestamp>@example.test` / `Strong1!` | 状态栏 `Registered.`；响应 `code=0`；设置 HttpOnly `cw_session`；返回 user 不含 password_hash。 |
| 08-03 | 注册邮箱前后带空格和大写。 | ` User<AuthTS>@Example.Test ` / `Strong1!` | 后端规范化为小写去空格；profile email 小写。 |
| 08-04 | 注册无效邮箱。 | `invalid-email` / `Strong1!` | 返回 400，message=`valid email is required`。 |
| 08-05 | 重复注册。 | 同一规范化邮箱 | 返回 400，message=`email already registered`。 |
| 08-06 | 逐个执行强密码边界矩阵 P-01 到 P-08。 | 弱密码 | 均返回 400；不创建用户；不设置有效 session。 |
| 08-07 | 注册请求非法 JSON。 | body=`{bad` | 返回 400，message=`invalid json body`。 |
| 08-08 | 注册接口错误方法。 | GET `/api/v1/auth/register` | 返回 405。 |

## 登录 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 08-09 | 注销当前用户。 | 点击 Logout | cookie 清除；状态 `Logged out.`。 |
| 08-10 | 正确邮箱但错误密码。 | `Wrong1!` | 返回 400，message=`invalid email or password`；不设置新 session。 |
| 08-11 | 不存在邮箱登录。 | `missing@example.test` / `Strong1!` | 返回同样的 `invalid email or password`，不泄露账号是否存在。 |
| 08-12 | 邮箱大小写和空格登录。 | ` USER@example.TEST ` / `Strong1!` | 成功登录到规范化用户。 |
| 08-13 | 正确登录。 | 注册邮箱 / `Strong1!` | 状态 `Logged in.`；设置新的 HttpOnly `cw_session`。 |
| 08-14 | 登录请求非法 JSON。 | body=`{bad` | 返回 400。 |
| 08-15 | disabled 用户登录。 | 将测试用户状态置 disabled | 返回 400，message=`user is disabled`；不设置 session。 |

## Session 与注销

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 08-16 | 登录后访问 profile。 | `GET /api/v1/user/profile` | HTTP 200，返回当前用户 profile。 |
| 08-17 | 删除 cookie 访问 profile。 | 无 `cw_session` | 返回 401。 |
| 08-18 | 篡改 cookie。 | `cw_session=invalid` | 返回 401。 |
| 08-19 | 注销后复用旧 cookie。 | 保存旧 cookie 再调用 API | 返回 401，session 已撤销。 |
| 08-20 | 多浏览器上下文登录同一账号。 | 两个上下文 | 均可独立登录；注销一个上下文不应误清另一个上下文 session，除非设计明确全局注销。 |
| 08-21 | session 过期。 | 设置短 TTL 或构造过期 session | 过期后用户 API 返回 401。 |

## 修改密码

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 08-22 | 未登录修改密码。 | POST `/api/v1/user/password` | 返回 401。 |
| 08-23 | 当前密码错误。 | current=`Wrong1!`，new=`Better1!` | 返回 400，message=`invalid current password`。 |
| 08-24 | 新密码弱密码。 | 强密码矩阵 P-01 到 P-08 | 每个均返回 400；旧密码仍有效。 |
| 08-25 | 新密码强密码。 | current=`Strong1!`，new=`Better1!` | 返回 200；注销后旧密码失败，新密码成功。 |
| 08-26 | 修改密码请求非法 JSON。 | body=`{bad` | 返回 400。 |

## 找回/重置密码

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 08-27 | 请求已存在用户重置密码。 | 邮箱 | 返回 `accepted=true`；开发模式可返回 reset_token。 |
| 08-28 | 请求不存在用户重置密码。 | `missing@example.test` | 仍返回 `accepted=true`，不泄露用户是否存在。 |
| 08-29 | 请求无效邮箱重置密码。 | `bad-email` | 返回 accepted 或受控成功，不泄露信息。 |
| 08-30 | 使用弱新密码确认重置。 | token + `weak` | 返回 400，强密码策略生效。 |
| 08-31 | 使用错误 token 确认重置。 | `bad-token` + `Better1!` | 返回 400，message=`invalid or expired reset token`。 |
| 08-32 | 使用正确 token 确认重置。 | token + `Better1!` | 返回 200；旧密码失败，新密码成功。 |
| 08-33 | 重复使用同一 token。 | 同 token 再确认 | 返回 400，token 已使用。 |
| 08-34 | 过期 token。 | 构造过期 token | 返回 400。 |

## 订阅与限制

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 08-35 | Free 用户 profile。 | 默认注册用户 | plan=`free`，limits.max_rules=`5`，limits.alert_history=`20`。 |
| 08-36 | Free 用户创建 5 条规则。 | 5 条不同规则 | 均成功。 |
| 08-37 | Free 用户创建第 6 条规则。 | 第 6 条 | 返回 403，message=`subscription rule limit exceeded`。 |
| 08-38 | Free 用户查询告警历史 limit=9999。 | `/api/v1/user/alerts?limit=9999` | 返回数量不超过 Free alert_history 20。 |
| 08-39 | Pro 用户限制。 | 将测试用户 plan=pro | max_rules=50，alert_history=100；第 51 条被拒绝。 |
| 08-40 | VIP 用户限制。 | 将测试用户 plan=vip | max_rules=200，alert_history=200；第 201 条被拒绝。 |
| 08-41 | 未知 plan。 | 构造 plan=`unknown` | 按 Free 默认限制处理。 |

## 鉴权分离与安全

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 08-42 | Bearer 访问用户 API。 | 仅 `Authorization: Bearer change-me` | 返回 401。 |
| 08-43 | Cookie 访问 Admin API。 | 仅 `cw_session` | 返回 401。 |
| 08-44 | 注册/登录响应检查。 | 响应 body | 不包含 password_hash、session token 原文、reset token，除开发重置接口按配置返回。 |
| 08-45 | Cookie 属性检查。 | Set-Cookie | 包含 `HttpOnly`，过期时间符合 session TTL；同源请求可携带。 |
| 08-46 | 错误日志检查。 | 服务日志 | 不打印明文密码和 session token。 |

## 通过标准

- 注册、登录、注销、改密、重置密码正向流程可用。
- 所有弱密码边界在注册、修改、重置三条路径均被拒绝。
- session 生命周期和 Cookie 安全属性符合预期。
- Free/Pro/VIP 限制由后端强制执行。
- Admin Bearer 与用户 session 权限不混用。

## 证据留存

- 浏览器注册/登录/改密截图。
- 强密码边界 API 响应表。
- Set-Cookie 属性截图，遮盖 cookie value。
- 订阅限制第 6/51/201 条失败响应。
- 日志脱敏检查截图。

## 清理事项

- 注销测试账号。
- 清除浏览器 cookie。
- 不保留 reset token、session token 明文。
