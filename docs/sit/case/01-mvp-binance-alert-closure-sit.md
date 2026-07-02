# SIT-01 MVP Binance 告警闭环全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[1]2026-05-20-crypto-watchtower-mvp-implementation.md`

## 测试目标

验证 MVP 基线能力在 Binance 单交易所模式下形成闭环：配置加载、健康检查、WebSocket/REST 采集、事件标准化、规则判断、告警持久化、Telegram 发送尝试、通知日志、Admin UI/API 查询。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- PostgreSQL 与 Redis 健康。
- `CW_BINANCE_ENABLED=true`，`CW_OKX_ENABLED=false` 或未启用 OKX。
- Telegram 可使用占位 token/chat id；真实发送成功不是本用例强制要求，但失败必须可观测且受控。

## 测试数据

| 名称 | 值 |
| --- | --- |
| Admin Token | `change-me` |
| 交易所 | `binance` |
| 交易对 | `BTCUSDT` |
| 大单阈值 | `1000`，用于测试环境更容易触发 |
| 窗口阈值 | `2000` |
| 规则类型 | `large_trade`、`large_trade_window`、`liquidation`、`funding_anomaly` |

## 环境与健康检查

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 01-01 | 启动本地栈。 | `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` | App、PostgreSQL、Redis 启动成功；脚本退出码为 0。 |
| 01-02 | 调用 `/health`。 | `GET /health` | HTTP 200；依赖状态包含 PostgreSQL/Redis；collector 状态至少包含 Binance。 |
| 01-03 | 停止或模拟 Redis 不可用后调用 `/health`。 | 临时断开 Redis 或使用等价 mock | `/health` 返回受控依赖错误；HTTP 服务仍响应；恢复 Redis 后状态恢复。 |
| 01-04 | 浏览器打开 `/admin`。 | 无 token | 页面可渲染；未输入 Bearer 时受保护数据加载失败但页面不崩溃。 |
| 01-05 | 输入正确 Bearer 并刷新。 | `change-me` | Admin 数据加载成功。 |

## Admin 规则配置 E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 01-06 | 创建 Binance 大单系统规则。 | exchange=`binance`，symbol=`BTCUSDT`，rule_type=`large_trade`，threshold=`1000`，window_sec=`60`，enabled=true | 保存成功；规则列表出现该记录。 |
| 01-07 | 创建窗口规则。 | rule_type=`large_trade_window`，threshold=`2000`，window_sec=`60` | 保存成功；列表展示 window 秒数。 |
| 01-08 | 创建无效阈值规则。 | threshold=`0` 或负数 | API 返回 400；页面显示保存失败；数据库不新增无效规则。 |
| 01-09 | 创建缺少交易对规则。 | symbol 为空 | 浏览器 required 或 API 400 阻止提交。 |
| 01-10 | 未带 Bearer 调用写规则接口。 | `POST /api/v1/rules` 无 Authorization | 返回 401；规则不写入。 |
| 01-11 | 错误 Bearer 调用写规则接口。 | Authorization=`Bearer wrong-token` | 返回 401；规则不写入。 |

## Binance 事件到告警闭环

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 01-12 | 等待 Binance public collector 接收交易事件。 | 观察 `/health` collector last_event 或 Admin Events | collector 不断更新，reconnects 不异常增长。 |
| 01-13 | 查询事件列表。 | `GET /api/v1/admin/events?exchange=binance&symbol=BTCUSDT&limit=20` | 仅返回 Binance/BTCUSDT 事件；数量不超过 limit。 |
| 01-14 | 等待或回放超过阈值的大单事件。 | notional > `1000` | 生成 `large_trade` 告警，`alerts` 中可查。 |
| 01-15 | 等待或回放窗口内多笔交易。 | 60 秒内累计 notional > `2000` | 生成 `large_trade_window` 告警；不会因窗口累计无限增长导致内存膨胀。 |
| 01-16 | 等待或回放强平事件。 | liquidation notional > 阈值 | 生成 `liquidation` 告警。 |
| 01-17 | 等待或回放 funding 异常。 | funding abs percent > 阈值 | 生成 `funding_anomaly` 告警。 |

## 通知闭环

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 01-18 | 使用占位 Telegram token 触发告警。 | mock 或真实事件 | 告警仍持久化；通知日志状态为 `failed` 或受控错误；App 不退出。 |
| 01-19 | 使用 Telegram 沙箱 token 触发告警。 | 沙箱 bot/chat | 通知日志状态为 `sent`；消息内容包含交易所、交易对、规则类型、阈值或名义金额。 |
| 01-20 | 查询通知列表。 | `GET /api/v1/admin/notifications?status=failed&limit=20` | 能按状态筛选；响应不暴露 bot token。 |
| 01-21 | Telegram 发送失败后继续触发下一条告警。 | 连续两条事件 | 第二条仍可处理；失败发送不会阻塞采集和规则判断。 |

## Admin 查询边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 01-22 | 按 exchange 过滤。 | `exchange=binance` | rules、alerts、events 均只返回 Binance。 |
| 01-23 | 按 symbol 过滤。 | `symbol=BTCUSDT` | 不返回其他交易对。 |
| 01-24 | 按 rule_type 过滤。 | `rule_type=large_trade` | alerts/rules 仅返回匹配规则类型。 |
| 01-25 | limit 边界。 | `limit=1`、`limit=200`、`limit=9999`、`limit=-1` | 正数按上限返回；非法或过大不导致大结果集失控。 |
| 01-26 | 事件为空时查询。 | 使用不存在 symbol=`NOPEUSDT` | 返回 `code=0` 和空数组，不返回 500。 |

## 通过标准

- Binance-only 模式健康，HTTP 服务、PostgreSQL、Redis 均可用。
- 规则正向写入和反向校验符合预期。
- 事件、告警、通知三类数据可在 Admin UI 和 API 查询。
- Telegram 成功和失败路径均被通知日志记录。
- 所有列表接口有上限，不出现一次性加载无边界数据。

## 证据留存

- `/health` 响应。
- Admin 规则创建截图。
- events/alerts/notifications API 响应。
- Telegram 沙箱消息或失败日志。
- 服务日志中无 panic、无无限重启。

## 清理事项

- 删除或禁用测试阈值很低的系统规则，避免影响后续测试。
- 恢复 Telegram 配置为后续用例所需状态。
