# SIT-03 双交易所 Smoke 与 Phase 3 准备度全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[3]2026-06-29-dual-exchange-smoke-and-phase3-readiness.md`

## 测试目标

验证 Binance-only、OKX-only、双交易所、单 collector 故障、依赖抖动和前端查询场景下系统仍保持可用，为 Phase 3 用户通知能力提供稳定基础。

## 模式矩阵

| 模式 | Binance | OKX | 预期 |
| --- | --- | --- | --- |
| Binance-only | 开启 | 关闭 | Binance collector 正常；OKX 不出现。 |
| OKX-only | 关闭 | 开启 | OKX collector 正常或受控外网错误；HTTP/API 可用。 |
| Dual | 开启 | 开启 | 两类 collector 并行，Admin 可按 exchange 筛选。 |
| No collector | 关闭 | 关闭 | App 可启动；健康检查说明无 collector 或列表为空；Admin/用户 API 可用。 |
| One failed | 一个正常 | 一个故障 | 正常 collector 不受故障 collector 阻塞。 |

## 启动 Smoke

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 03-01 | 执行默认 smoke。 | `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` | 脚本通过；`/health`、读接口、写接口鉴权均通过。 |
| 03-02 | Binance-only smoke。 | `CW_BINANCE_ENABLED=true`，`CW_OKX_ENABLED=false` | `/health` 仅包含 Binance collector；Admin 可加载。 |
| 03-03 | OKX-only smoke。 | `CW_BINANCE_ENABLED=false`，`CW_OKX_ENABLED=true` | `/health` 包含 OKX collector；若外网不可达为受控错误。 |
| 03-04 | Dual smoke。 | 两者均开启 | 两类 collector 状态可见；`/api/v1/admin/events?exchange=binance` 和 `exchange=okx` 均可查询。 |
| 03-05 | No collector smoke。 | 两者均关闭 | App 不退出；用户注册/登录/API 仍可用。 |

## 浏览器稳定性 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 03-06 | 打开 `/admin` 并输入正确 Bearer。 | `change-me` | 概览、运行状态、趋势、列表面板均渲染；无 JS error。 |
| 03-07 | Admin 快速切换筛选器。 | exchange 在 Binance/OKX/Any 间切换，limit 1/8/200 | 不出现错乱数据；请求有界；页面不卡死。 |
| 03-08 | 打开 `/dashboard`，注册用户。 | `sit-phase3-ready-<timestamp>@example.test`，`Strong1!` | 登录态建立；用户面板加载。 |
| 03-09 | Dashboard 创建 Binance 与 OKX 各一条个人规则。 | 两条规则同 symbol 不同 exchange | 列表展示两条规则；互不覆盖。 |
| 03-10 | Dashboard 反复点击 Reload。 | 连续 5 次 | 数据稳定，状态从 Loading 回到 Loaded；无未处理异常。 |

## 故障注入

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 03-11 | OKX WS 指向不可达地址，Binance 保持正常。 | `CW_OKX_PUBLIC_WS_BASE_URL=ws://127.0.0.1:1` | OKX 状态失败或重连；Binance 事件继续进入；HTTP 服务可用。 |
| 03-12 | Binance WS 指向不可达地址，OKX 保持正常。 | mock 不可达 Binance endpoint | Binance 状态失败；OKX 不受影响。 |
| 03-13 | 临时断开 PostgreSQL。 | 停止数据库容器或使用等价 mock | `/health` 依赖状态异常；接口返回受控 500；App 不 panic。 |
| 03-14 | 恢复 PostgreSQL。 | 启动数据库容器 | `/health` 恢复；列表接口恢复。 |
| 03-15 | 临时断开 Redis。 | 停止 Redis 容器或 mock | 健康检查报告异常；不影响静态页面渲染。 |
| 03-16 | 快速重启 App。 | 停止再启动 App 容器 | session 是否失效按设计处理；服务恢复后 `/admin` 和 `/dashboard` 可访问。 |

## API 边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 03-17 | Admin 无 token 查询所有列表。 | overview/trends/rules/alerts/events/notifications | 均返回 401；`/health` 不需要 token。 |
| 03-18 | Admin 错误 token 查询所有列表。 | `Bearer bad` | 均返回 401。 |
| 03-19 | Admin 正确 token 查询所有列表。 | `Bearer change-me` | 均返回 `code=0`。 |
| 03-20 | 过滤不存在 exchange。 | `exchange=nope` | 空结果或受控响应；不返回 500。 |
| 03-21 | 过滤 limit 非数字。 | `limit=abc` | 使用默认上限或受控处理；不返回大结果。 |
| 03-22 | 过滤 user_id 非法。 | `user_id=-1`、`user_id=abc` | Admin 过滤忽略非法 user_id 或受控处理；用户 API 不允许越权。 |

## Phase 3 准备度检查

| 编号 | 检查项 | 预期结果 |
| --- | --- | --- |
| 03-23 | 用户 session API 可用。 | 注册、登录、profile、logout 均可用。 |
| 03-24 | 用户规则 API 可用。 | `/api/v1/user/rules` 按 session 隔离。 |
| 03-25 | Telegram binding-token API 可用。 | 登录后可生成短期 token，未登录返回 401。 |
| 03-26 | Telegram delivery API 可用。 | 登录后可开关投递，未登录返回 401。 |
| 03-27 | 用户通知日志 API 可用。 | 登录后返回有界列表，未登录返回 401。 |
| 03-28 | Dashboard 静态资产可用。 | `/dashboard`、`/dashboard/app.js`、`/dashboard/styles.css` 均可访问。 |

## 通过标准

- 五种启动模式结果符合模式矩阵。
- 单 collector 故障不拖垮 HTTP、Admin、Dashboard 或另一 collector。
- 所有列表接口有边界。
- Phase 3 所需用户 API 全部可访问并具备 session 鉴权。

## 证据留存

- 每种模式 `/health` 响应。
- Admin 筛选截图。
- Dashboard 注册和规则创建截图。
- 故障注入前后服务日志。
- smoke 命令输出。

## 清理事项

- 恢复默认 collector 配置。
- 删除或禁用低阈值测试规则。
