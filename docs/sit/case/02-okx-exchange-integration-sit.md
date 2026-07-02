# SIT-02 OKX 第二交易所接入全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[2]2026-06-29-okx-exchange-integration.md`

## 测试目标

验证 OKX 作为第二交易所端到端可用：配置开关、instrument id 映射、Public WebSocket collector、REST metadata、事件标准化、规则判断、Admin 过滤、与 Binance 数据隔离。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- `CW_OKX_ENABLED=true`。
- 外网可访问 OKX public endpoint；如果不可访问，使用自动化测试样例 payload 或本地 mock WebSocket/REST。
- Binance 可同时开启，用于验证跨交易所隔离。

## 测试数据

| 名称 | 值 |
| --- | --- |
| OKX 紧凑交易对 | `BTCUSDT` |
| OKX instrument id | spot=`BTC-USDT`，swap=`BTC-USDT-SWAP` |
| 规则类型 | `large_trade`、`large_trade_window`、`liquidation`、`funding_anomaly` |
| Admin Token | `change-me` |

## 配置与启动

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 02-01 | 以 OKX-only 启动。 | `CW_BINANCE_ENABLED=false`，`CW_OKX_ENABLED=true` | App 启动成功；`/health` 包含 OKX collector；无 Binance collector。 |
| 02-02 | 以双交易所启动。 | `CW_BINANCE_ENABLED=true`，`CW_OKX_ENABLED=true` | 两类 collector 均可见；HTTP 服务健康。 |
| 02-03 | 配置无效 OKX WS URL。 | `CW_OKX_PUBLIC_WS_BASE_URL=ws://127.0.0.1:1` | App 不崩溃；collector 状态显示受控错误或重连；Admin/用户 API 仍可用。 |
| 02-04 | 配置空 OKX symbols。 | 空或未设置 symbols | 使用默认或配置内 symbols；若无 symbols，collector 不应 panic。 |

## Instrument 与事件标准化

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 02-05 | 查询或 mock OKX instrument metadata。 | spot/swap metadata | `BTCUSDT` 能映射到 `BTC-USDT` 或 `BTC-USDT-SWAP`。 |
| 02-06 | 回放 OKX trade payload。 | OKX trade 样例 | 标准化事件 exchange=`okx`，symbol=`BTCUSDT`，event_type 为 trade/agg_trade 等项目约定类型，notional 可计算。 |
| 02-07 | 回放 OKX liquidation payload。 | OKX liquidation 样例 | 标准化为 liquidation 类事件；side、price、qty、notional 不为空或有合理默认。 |
| 02-08 | 回放 OKX funding payload。 | funding rate 样例 | 标准化为 funding 异常可判断的数据。 |
| 02-09 | 回放缺字段 payload。 | 缺 price、qty、instId 或时间字段 | 记录受控解析错误或丢弃该事件；服务不 panic；不写入脏事件。 |

## OKX 告警闭环

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 02-10 | Admin 创建 OKX 大单规则。 | exchange=`okx`，symbol=`BTCUSDT`，rule_type=`large_trade`，threshold=`1000` | 保存成功；规则 scope 为 system，exchange 为 okx。 |
| 02-11 | Dashboard 创建 OKX 个人规则。 | 登录用户，exchange=`okx`，symbol=`BTCUSDT`，threshold=`1000` | 保存成功；只归属当前用户。 |
| 02-12 | 触发 OKX 大单事件。 | notional > threshold | 生成 OKX 告警；通知日志关联该告警。 |
| 02-13 | 触发 Binance 同 symbol 事件。 | exchange=`binance`，symbol=`BTCUSDT` | 不误命中 OKX-only 规则；Admin OKX 过滤不出现 Binance 告警。 |
| 02-14 | 触发 OKX 窗口规则。 | 多事件累计超过阈值 | 在窗口内生成告警；窗口数据按 exchange+symbol+rule 隔离。 |

## Admin 过滤与 UI E2E

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 02-15 | `/admin` 输入正确 Bearer 刷新。 | `change-me` | OKX collector 状态可见。 |
| 02-16 | 筛选交易所为 OKX。 | `filter-exchange=okx` | Rules、Alerts、Events 显示 OKX 数据；无 Binance-only 记录。 |
| 02-17 | 筛选交易所为 Binance。 | `filter-exchange=binance` | 不出现 OKX-only 记录。 |
| 02-18 | 筛选 symbol 小写输入。 | `btcusdt` | 前端转大写请求；返回 `BTCUSDT` 数据。 |
| 02-19 | API 直接查询 OKX 事件。 | `/api/v1/admin/events?exchange=okx&limit=20` | HTTP 200，数量有上限，记录 exchange 均为 okx。 |
| 02-20 | API 直接查询 OKX 告警。 | `/api/v1/admin/alerts?exchange=okx&limit=20` | HTTP 200；如有数据，exchange 均为 okx。 |

## 反向与边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 02-21 | 无 Bearer 查询 Admin OKX 数据。 | 不带 Authorization | 返回 401。 |
| 02-22 | 错误 exchange 查询。 | `exchange=unknown` | 返回空数组或受控结果，不返回 500。 |
| 02-23 | OKX 网络中断。 | 断网或 mock server 关闭 | collector 进入受控错误/重连；已有 API 可用；不会影响 PostgreSQL/Redis。 |
| 02-24 | OKX 恢复网络。 | 恢复 endpoint | collector 可恢复接收事件；reconnects 计数合理增加。 |
| 02-25 | 大批量 OKX 事件。 | 连续回放 1000 条样例 | 列表接口仍按 limit 返回；服务内存无明显持续增长。 |

## 通过标准

- OKX-only 和双交易所模式均可启动。
- OKX 事件标准化、告警、通知、Admin 查询闭环可用。
- OKX 与 Binance 规则和数据按 exchange 隔离。
- OKX 网络异常、payload 异常均为受控失败。

## 证据留存

- OKX 启动环境变量截图或命令。
- `/health` collector 状态。
- OKX events/alerts/rules API 响应。
- Admin OKX 筛选截图。
- mock payload 与处理结果日志。

## 清理事项

- 如果后续用例使用 Binance-only，关闭 `CW_OKX_ENABLED`。
- 删除测试 OKX 低阈值规则。
