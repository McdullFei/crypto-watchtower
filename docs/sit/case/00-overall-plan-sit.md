# SIT-00 总体开发计划全量 E2E 测试用例

## 来源

- `docs/plan/币圈异动监控平台总体开发计划.md`
- 覆盖计划：总体计划、计划 [1] 到计划 [13]。

## 测试目标

验证 CryptoWatchtower 作为完整系统在浏览器、HTTP API、持久化、通知和权限边界上的端到端可用性。该用例作为全量回归入口，要求串联 Admin、Dashboard、认证、订阅、规则、告警、Telegram 绑定、投递偏好、免打扰、摘要和通知日志。

## 执行约定

- 基础地址：`http://127.0.0.1:18080`
- Admin Bearer Token：`change-me`
- 浏览器建议使用无痕窗口或清空站点数据后执行。
- 所有账号密码测试必须遵守强密码策略：至少 8 位，且包含大写字母、小写字母、数字、特殊字符，不允许空白字符。
- 涉及 Telegram、Discord/Webhook、AI 服务时，允许使用 mock server、沙箱 bot 或占位配置；必须验证失败路径是受控失败，不允许服务崩溃。
- 所有接口响应中不得记录或截图原始 `cw_session`、明文密码、完整 Telegram chat id、真实 bot token、真实 webhook URL。

## 测试数据

| 名称 | 值 |
| --- | --- |
| 用户 A 邮箱 | `sit-overall-a-<timestamp>@example.test` |
| 用户 B 邮箱 | `sit-overall-b-<timestamp>@example.test` |
| 正确密码 | `Strong1!` |
| 修改后密码 | `Better1!` |
| 弱密码样例 | `short1!`、`password1!`、`PASSWORD1!`、`Password!`、`Password1`、`Pass word1!` |
| 交易对 | `BTCUSDT`、`ETHUSDT` |
| 交易所 | `binance`、`okx` |
| 规则类型 | `large_trade`、`large_trade_window`、`liquidation`、`funding_anomaly` |

## 环境启动与健康检查

| 编号 | 操作 | 预期结果 | 证据 |
| --- | --- | --- | --- |
| 00-01 | 执行 `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh`。 | Compose 配置通过，PostgreSQL、Redis、App 启动；脚本内健康检查通过。 | 保存命令输出。 |
| 00-02 | 浏览器打开 `/admin`。 | Admin UI 首屏渲染，能看到 Bearer Token、筛选器、运行状态、规则编辑区域。 | 页面截图。 |
| 00-03 | 浏览器打开 `/dashboard`，不要登录。 | 页面渲染；状态栏先显示加载失败或未登录提示；规则、告警、通知区域为空或清空。 | 页面截图和控制台无 JS error。 |
| 00-04 | 调用 `GET /health`。 | HTTP 200，`code=0`，包含依赖状态和 collector 状态。 | JSON 响应。 |

## Admin E2E 场景

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 00-05 | `/admin` 中 Bearer Token 留空，点击“刷新”。 | 空 token | 运行状态可从 `/health` 渲染；Admin overview/rules/alerts/events/notifications 请求失败，状态栏展示加载失败。 |
| 00-06 | Bearer Token 输入错误值，点击“刷新”。 | `wrong-token` | 受保护 Admin API 返回 401；页面不展示受保护数据；不跳转、不崩溃。 |
| 00-07 | Bearer Token 输入正确值，点击“刷新”。 | `change-me` | overview、趋势、规则、事件、告警、通知区域加载成功；状态栏显示“后台数据已加载”或等价中文。 |
| 00-08 | 切换语言为中文，再刷新。 | 语言选择 `中文` | 页面静态文案保持中文，刷新后语言不丢失。 |
| 00-09 | 使用 Admin 规则编辑创建系统规则。 | exchange=`binance`，symbol=`BTCUSDT`，rule_type=`large_trade`，threshold=`100000`，window_sec=`60`，enabled=true | 状态栏显示规则已保存；`/api/v1/admin/rules?symbol=BTCUSDT&limit=20` 能查到该系统规则。 |
| 00-10 | Admin 筛选 limit 边界。 | limit=`1`、`200`、`999` | 页面请求不报错；列表不超过后端上限；过大 limit 不导致页面卡死或大结果集一次性渲染失控。 |

## Dashboard 认证 E2E 场景

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 00-11 | 注册用户 A。 | 邮箱用户 A，密码 `Strong1!` | 状态栏显示 `Registered.`；浏览器收到 HttpOnly `cw_session`；Dashboard 自动加载 profile/rules/alerts/notifications。 |
| 00-12 | 重复注册用户 A。 | 同一邮箱、同一密码 | 注册失败，状态栏包含 `email already registered`；原 session 不应被替换为新用户。 |
| 00-13 | 注册弱密码账号。 | 依次输入弱密码样例 | 每个样例均注册失败；错误分别覆盖长度不足、缺少大小写/数字/特殊字符、包含空白字符。 |
| 00-14 | 注销用户 A。 | 点击 Logout | 状态栏显示 `Logged out.`；页面列表清空；再调用用户 API 返回 401。 |
| 00-15 | 登录错误密码。 | 用户 A 邮箱，`Wrong1!` | 登录失败，状态栏包含 `invalid email or password`；不设置有效 session。 |
| 00-16 | 登录正确密码。 | 用户 A 邮箱，`Strong1!` | 登录成功，Dashboard 加载用户 A 数据。 |
| 00-17 | 修改密码时输入错误当前密码。 | current=`Wrong1!`，new=`Better1!` | 修改失败，状态栏包含 `invalid current password`；旧密码仍可登录。 |
| 00-18 | 修改密码为弱密码。 | current=`Strong1!`，new=`weak` | 修改失败，错误体现强密码策略；旧密码仍可登录。 |
| 00-19 | 修改密码为强密码。 | current=`Strong1!`，new=`Better1!` | 修改成功；注销后旧密码登录失败，新密码登录成功。 |

## Dashboard 业务 E2E 场景

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 00-20 | 创建个人规则。 | exchange=`binance`，symbol=`BTCUSDT`，rule_type=`large_trade`，threshold=`1000`，window_sec=`60`，enabled=true | `Personal Rules` 列表出现 `BTCUSDT · large_trade`；接口响应的规则归属当前 session 用户。 |
| 00-21 | 创建 OKX 个人规则。 | exchange=`okx`，symbol=`BTCUSDT`，rule_type=`large_trade_window`，threshold=`2000`，window_sec=`120` | 列表出现 OKX 规则；Admin 按 `exchange=okx` 能筛到相同规则。 |
| 00-22 | 创建无效规则。 | 空 symbol、threshold=`0`、window_sec=`0` 分别提交 | 前端 required 或后端 400 阻止保存；不会产生脏规则。 |
| 00-23 | 生成 Telegram 绑定 token。 | 点击 Create Token | 页面显示 `/start <token>` 和过期时间；token 不写入日志截图。 |
| 00-24 | 关闭 Telegram delivery。 | 取消勾选 `Telegram delivery enabled` | 状态显示 disabled；绑定状态不被删除。 |
| 00-25 | 开启 Telegram delivery。 | 勾选 `Telegram delivery enabled` | 状态恢复 enabled；profile 返回 `telegram_delivery_enabled=true`。 |
| 00-26 | 保存免打扰配置。 | enabled=true，start=`22:00`，end=`08:00`，timezone=`Asia/Shanghai` | 保存成功；刷新后配置仍存在。 |
| 00-27 | 保存摘要配置。 | digest=true，interval=`60` | 保存成功；刷新后配置仍存在。 |
| 00-28 | 免打扰/摘要边界。 | timezone=`Mars/Phobos`、start=`24:00`、end=`7:00`、interval=`4`、`1441` | 每个非法输入均被 400 拒绝；页面显示失败；原有效配置不被覆盖。 |

## 用户隔离与安全边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 00-29 | 注册并登录用户 B。 | 用户 B 邮箱，`Strong1!` | 用户 B Dashboard 加载成功。 |
| 00-30 | 用户 B 查看个人规则。 | 进入 `/dashboard` 或调用 `/api/v1/user/rules` | 不出现用户 A 的规则。 |
| 00-31 | 用户 B 尝试通过 query 越权读取用户 A。 | `/api/v1/user/rules?user_id=<A>`、`/profile?user_id=<A>`、`/notifications?user_id=<A>` | 后端忽略 query user_id，仅返回用户 B 数据。 |
| 00-32 | 未登录访问用户 API。 | 删除 cookie 后调用 `/api/v1/user/profile` | 返回 401；不返回用户数据。 |
| 00-33 | 用 session cookie 访问 Admin API。 | 无 Bearer，仅带 cookie 调用 `/api/v1/admin/overview` | 返回 401；Admin 权限与用户 session 分离。 |
| 00-34 | 用 Bearer 访问用户 API。 | 无 cookie，仅带 Bearer 调用 `/api/v1/user/profile` | 返回 401；用户 API 不接受 Bearer 替代 session。 |

## 通过标准

- 所有正向浏览器场景成功，所有反向/边界场景给出受控错误。
- Admin 与 Dashboard 权限模型相互隔离。
- 用户数据按 session 隔离，不可通过 query 参数越权。
- 强密码策略在注册、重置、修改密码路径均被后端强校验。
- 通知相关敏感字段均脱敏或不暴露。
- 任何 mock 外部服务不可用时，系统只记录失败状态，不崩溃、不无限重试。

## 证据留存

- 启动和 smoke 命令输出。
- Admin 成功/失败 token 场景截图。
- Dashboard 注册、登录、错误密码、弱密码、规则创建、Telegram 偏好截图。
- 关键 API JSON：`/health`、`/api/v1/user/profile`、`/api/v1/user/rules`、`/api/v1/admin/overview`。
- 浏览器控制台截图，确认无未处理 JS error。

## 清理事项

- 注销测试用户。
- 如后续用例复用环境，保留服务；否则停止 Docker Compose。
- 不保留真实 token、chat id、webhook URL 或 session cookie 证据。
