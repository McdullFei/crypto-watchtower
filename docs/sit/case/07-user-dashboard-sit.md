# SIT-07 用户侧 Dashboard 全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[7]2026-06-30-user-dashboard.md`

## 测试目标

验证 `/dashboard` 用户侧页面的浏览器端到端体验：静态资源、注册、登录、注销、改密、profile、订阅展示、Telegram 绑定 token、投递开关、免打扰/摘要表单、个人规则、告警历史、通知日志、错误提示和响应式稳定性。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- 使用新浏览器上下文，避免旧 cookie 干扰。
- 用户 API、认证 API 已可用。

## 页面元素清单

| 区域 | 必须存在的元素 ID |
| --- | --- |
| 登录 | `login-form`、`login-email`、`login-password` |
| 注册 | `register-form`、`register-email`、`register-password` |
| 改密 | `password-form`、`current-password`、`new-password` |
| 会话 | `reload-button`、`logout-button`、`status-text` |
| Telegram | `telegram-bind-button`、`telegram-binding-token`、`telegram-unbind-button`、`telegram-delivery-enabled` |
| 偏好 | `telegram-preferences-form`、`telegram-quiet-hours-enabled`、`telegram-quiet-hours-start`、`telegram-quiet-hours-end`、`telegram-quiet-hours-timezone`、`telegram-digest-enabled`、`telegram-digest-interval` |
| 规则/列表 | `rule-form`、`rule-exchange`、`rule-symbol`、`rule-type`、`rule-threshold`、`rule-window-sec`、`rules-table`、`alerts-table`、`notifications-table` |

## 静态页面与未登录状态

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 07-01 | 浏览器打开 `/dashboard`。 | 无 cookie | HTML 正常渲染；上述元素均存在。 |
| 07-02 | 检查 JS/CSS 加载。 | `/dashboard/app.js`、`/dashboard/styles.css` | HTTP 200；控制台无资源 404。 |
| 07-03 | 首次自动 loadDashboard。 | 无 cookie | 状态栏显示 `Load failed` 或等价错误；列表清空为 `No data` 或空态。 |
| 07-04 | 未登录点击 Reload。 | 无 cookie | 不跳转；继续显示未登录错误。 |
| 07-05 | 未登录点击 Create Token。 | 无 cookie | 状态栏显示 Telegram binding failed；API 返回 401。 |
| 07-06 | 未登录提交规则。 | 填完整规则 | 状态栏显示 Save failed；API 返回 401。 |

## 注册与登录浏览器 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 07-07 | 注册强密码用户。 | `sit-dashboard-<timestamp>@example.test` / `Strong1!` | 状态栏 `Registered.`；profile、rules、alerts、notifications 自动加载。 |
| 07-08 | 注册弱密码。 | `short1!` | 状态栏 `Register failed`，包含长度错误。 |
| 07-09 | 注册缺少大写。 | `password1!` | 注册失败，包含复杂度错误。 |
| 07-10 | 注册缺少小写。 | `PASSWORD1!` | 注册失败。 |
| 07-11 | 注册缺少数字。 | `Password!` | 注册失败。 |
| 07-12 | 注册缺少特殊字符。 | `Password1` | 注册失败。 |
| 07-13 | 注册包含空白字符。 | `Pass word1!` | 注册失败。 |
| 07-14 | 注销后错误密码登录。 | 正确邮箱 / `Wrong1!` | 状态栏 `Login failed: invalid email or password`。 |
| 07-15 | 正确密码登录。 | 正确邮箱 / `Strong1!` | 状态栏 `Logged in.`；Dashboard 显示订阅和 Telegram 状态。 |

## 改密 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 07-16 | 未登录改密。 | 删除 cookie 后提交 | API 返回 401；状态栏 Change failed。 |
| 07-17 | 错误当前密码。 | current=`Wrong1!`，new=`Better1!` | 修改失败；旧密码仍可登录。 |
| 07-18 | 新密码弱密码边界。 | 依次使用弱密码样例 | 每个均失败，后端强校验。 |
| 07-19 | 新密码强密码。 | current=`Strong1!`，new=`Better1!` | 状态栏 `Password changed.`；注销后旧密码失败，新密码成功。 |

## Profile 与订阅展示

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 07-20 | 登录后查看订阅。 | 默认 free 用户 | `subscription-status` 显示 `Subscription: free`；`plan-limits` 显示 Plan Rules=5、Alert History=20。 |
| 07-21 | 刷新 Dashboard。 | 点击 Reload | profile/rules/alerts/notifications 并发加载成功；状态栏 `Loaded.`。 |
| 07-22 | 后端返回空列表。 | 新用户无规则/告警/通知 | 三个列表显示空态，不报 JS error。 |
| 07-23 | profile 含最近投递状态。 | 构造通知日志 | `telegram-delivery-status` 展示 recent status。 |

## Telegram 与偏好表单

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 07-24 | 点击 Create Token。 | 登录状态 | `telegram-binding-token` 显示 `/start <token>`；过期时间显示。 |
| 07-25 | 点击 Unbind。 | 登录状态 | 状态栏 `Telegram unbound.`；绑定状态显示 not bound；投递偏好不被重置。 |
| 07-26 | 关闭 delivery。 | 取消勾选 | 状态栏 `Telegram delivery updated.`；delivery status 显示 disabled。 |
| 07-27 | 开启 delivery。 | 勾选 | 状态恢复 enabled。 |
| 07-28 | 保存免打扰。 | enabled=true，22:00-08:00，Asia/Shanghai | 保存成功；Reload 后仍显示。 |
| 07-29 | 保存摘要。 | enabled=true，60 分钟 | 保存成功；Reload 后仍显示。 |
| 07-30 | 非法 timezone。 | `Mars/Phobos` | 状态栏 `Telegram preference update failed`；原配置保持。 |
| 07-31 | 非法时间。 | 通过 API 或浏览器脚本传 `24:00`、`7:00` | 返回 400；状态栏失败。 |
| 07-32 | 非法摘要分钟。 | `4`、`1441` | 返回 400；边界 `5` 和 `1440` 可成功。 |

## 规则、告警、通知列表

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 07-33 | 创建个人规则。 | `BTCUSDT large_trade threshold=1000 window=60` | 规则列表出现记录。 |
| 07-34 | 创建 OKX 规则。 | exchange=`okx` | 规则列表展示 okx。 |
| 07-35 | 规则表单空 symbol。 | 清空 symbol | 浏览器阻止提交或 API 400。 |
| 07-36 | threshold 为 0。 | `0` | 保存失败。 |
| 07-37 | 触发用户告警后刷新。 | mock event | Alert History 出现用户告警。 |
| 07-38 | 触发通知后刷新。 | mock send | Notification Logs 出现 channel/status/target/alert id；target 脱敏或安全展示。 |
| 07-39 | 列表 limit。 | API 请求 `limit=1`、`limit=200`、`limit=9999` | 返回数量有界；页面渲染不卡死。 |

## 响应式与稳定性

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 07-40 | 桌面视口截图。 | 1440x900 | 无元素重叠，主要控件可见。 |
| 07-41 | 移动视口截图。 | 390x844 | 文本不溢出按钮/表单；可滚动访问所有区域。 |
| 07-42 | 连续快速点击 Reload。 | 5 次 | 最终状态稳定，控制台无未处理 Promise error。 |
| 07-43 | 后端临时返回 500。 | mock 或断开依赖 | 页面显示失败并清空旧数据，不展示误导性旧状态。 |

## 通过标准

- Dashboard 所有核心表单可从浏览器完成正向流程。
- 未登录、弱密码、错误密码、非法偏好、非法规则均被受控拒绝。
- 页面不暴露原始 session token、完整 chat id、明文密码。
- 桌面和移动视口无明显布局破坏。

## 证据留存

- 注册/登录/改密/Telegram/规则创建截图。
- 弱密码错误状态截图。
- 浏览器控制台无错误截图。
- profile、rules、notifications API JSON。

## 清理事项

- 注销测试用户。
- 清空浏览器站点数据或使用新上下文。
