# SIT-02 OKX 第二交易所接入全量 E2E 测试报告

## 基本信息

- 测试日期：2026-07-02
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 测试用例：`docs/sit/case/02-okx-exchange-integration-sit.md`
- Admin Token：`change-me`
- 测试技能：Build Web Apps:frontend-testing-debugging、superpowers:test-driven-development、superpowers:systematic-debugging、Playwright CLI

## 总体结论

SIT-02 最终通过。

- OKX-only、双交易所、无效 OKX WS URL、空 OKX symbols 默认值均已验证。
- OKX instrument metadata、trade/liquidation/funding 标准化、缺字段/错误 ack 受控路径均由自动化测试覆盖并通过。
- OKX 系统规则、Dashboard 个人规则、OKX 告警闭环、通知日志、Admin/API/UI 过滤、Binance/OKX 隔离均已验证。
- 本轮未发现需要新增代码修复的 SIT-02 bug。

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 02-01 | 通过 | `CW_BINANCE_ENABLED=false`、`CW_OKX_ENABLED=true` 启动成功；`/health` 仅包含 `okx-public`，无 Binance collector。 |
| 02-02 | 通过 | 双交易所启动成功；`/health` 同时包含 `binance-spot`、`binance-futures`、`okx-public`。 |
| 02-03 | 通过 | `CW_OKX_PUBLIC_WS_BASE_URL=ws://127.0.0.1:1` 时服务不崩溃；OKX collector 显示 `connection refused` 与重连；Admin API 正常。 |
| 02-04 | 通过 | `CW_OKX_SYMBOLS=` 启动后使用 compose 默认 `BTCUSDT`；服务无 panic。 |
| 02-05 | 通过 | `TestOKXInstrumentSymbolMapping`、`TestOKXInstrumentNotional`、`TestOKXInstrumentFetcherParsesPublicInstruments` 通过。 |
| 02-06 | 通过 | `TestOKXNormalizeTrades` 通过；OKX trade 标准化为 `exchange=okx`、`symbol=BTCUSDT`、`event_type=agg_trade`。 |
| 02-07 | 通过 | `TestOKXNormalizeLiquidations` 通过；liquidation side、price、qty、notional 可计算。 |
| 02-08 | 通过 | `TestOKXNormalizeFunding` 通过；funding rate 转为可判断的 `funding_rate` metadata。 |
| 02-09 | 通过 | OKX normalizer 缺字段会返回受控解析错误；`TestOKXWSRecordsErrorAck` 验证交易所错误 ack 被记录到 collector status。 |
| 02-10 | 通过 | Admin 创建 OKX `large_trade` 系统规则成功，`scope=system`、`exchange=okx`。 |
| 02-11 | 通过 | Dashboard 注册用户 `sit02-20260702@example.test` 后创建 OKX 个人规则成功，`scope=user`、`UserID=24`。 |
| 02-12 | 通过 | 回放 `sit02-okx-large-20260702-1`，生成 OKX `large_trade` 告警并记录通知日志。 |
| 02-13 | 通过 | 回放 Binance 同 symbol 事件；OKX 过滤结果不出现 Binance 记录，Binance 过滤结果不出现 `sit02-okx-*` 告警。 |
| 02-14 | 通过 | OKX 大单回放同时触发 `large_trade_window`，窗口累计 `2500 > 2000`。 |
| 02-15 | 通过 | Playwright 打开 `/admin`，输入 `change-me` 后刷新，OKX collector 状态可见。 |
| 02-16 | 通过 | UI 筛选交易所 OKX 后，Rules、Alerts、Events 显示 OKX 数据。 |
| 02-17 | 通过 | UI 筛选交易所 Binance 后，不显示本次 `sit02-okx-*` 记录。 |
| 02-18 | 通过 | UI 输入小写 `btcusdt` 后刷新，返回 `BTCUSDT` 数据，验证前端请求时转大写。 |
| 02-19 | 通过 | `GET /api/v1/admin/events?exchange=okx&symbol=BTCUSDT&limit=50` 返回 3 条本次 OKX replay 事件，均为 `exchange=okx`。 |
| 02-20 | 通过 | `GET /api/v1/admin/alerts?exchange=okx&symbol=BTCUSDT&limit=50` 返回 4 条本次 OKX replay 告警，均为 `exchange=okx`。 |
| 02-21 | 通过 | 无 Bearer 查询 Admin OKX 数据返回 HTTP 401。 |
| 02-22 | 通过 | `exchange=unknown` 返回 `code=0` 与空数组 `data=[]`，无 500。 |
| 02-23 | 通过 | 无效 OKX WS URL 模拟网络中断，collector 进入受控错误/重连，PostgreSQL/Redis 与 API 正常。 |
| 02-24 | 通过 | 恢复有效 OKX endpoint 后服务健康；通过 replay 路径继续接收 OKX 事件。 |
| 02-25 | 通过 | 连续 replay 1000 条 OKX 低名义金额样例成功；列表接口仍按 limit 返回；内存从约 `25.75MiB` 到 `27.2MiB`，无明显持续增长。 |

## Bug 与复测

- 本轮 SIT-02 未发现需要代码修复的新 bug。
- 复测命令均通过：

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/collector -run OKX -v && /usr/local/go/bin/go test ./cmd/server -run BuildMarketCollectors -v && /usr/local/go/bin/go test ./internal/api ./internal/rule -v'
```

## 关键证据

- OKX-only health：collector 仅 `okx-public`，subscribed=`BTCUSDT`。
- 双交易所 health：`binance-spot`、`binance-futures`、`okx-public` 均可见。
- 无效 WS URL health：`okx-public.last_error=dial tcp 127.0.0.1:1: connect: connection refused`，API 仍返回 `code=0`。
- OKX replay events：`sit02-okx-large-20260702-1`、`sit02-okx-liquidation-20260702-1`、`sit02-okx-funding-20260702-1`。
- OKX replay alerts：`large_trade`、`large_trade_window`、`liquidation`、`funding_anomaly` 四类均出现。
- OKX sent notifications：本次 `sit02-okx-*` 告警均有 `telegram/default/sent` 通知日志。
- 1000 条 replay 后 `/api/v1/admin/events?exchange=okx&limit=20` 返回有界结果，且结果均为 OKX。

## 环境恢复说明

- 已清理 Playwright 临时目录 `.playwright-cli/`。
- 低阈值 OKX 系统规则和测试用户 OKX 个人规则属于本轮测试数据，不影响 SIT-02 用例通过结论。
- 如需恢复到测试前规则状态，可在具备本地接口写入权限时执行：

```bash
curl -H 'Authorization: Bearer change-me' -H 'Content-Type: application/json' -X POST http://127.0.0.1:18080/api/v1/rules -d '{"exchange":"okx","symbol":"BTCUSDT","rule_type":"large_trade","threshold":1000,"window_sec":60,"enabled":false}'
curl -H 'Authorization: Bearer change-me' -H 'Content-Type: application/json' -X POST http://127.0.0.1:18080/api/v1/rules -d '{"exchange":"okx","symbol":"BTCUSDT","rule_type":"large_trade_window","threshold":2000,"window_sec":60,"enabled":false}'
curl -H 'Authorization: Bearer change-me' -H 'Content-Type: application/json' -X POST http://127.0.0.1:18080/api/v1/rules -d '{"exchange":"okx","symbol":"BTCUSDT","rule_type":"liquidation","threshold":1000,"window_sec":60,"enabled":false}'
curl -H 'Authorization: Bearer change-me' -H 'Content-Type: application/json' -X POST http://127.0.0.1:18080/api/v1/rules -d '{"exchange":"okx","symbol":"BTCUSDT","rule_type":"funding_anomaly","threshold":0.08,"window_sec":60,"enabled":false}'
```
