# SIT-05 AI 市场摘要全量 E2E 测试用例

## 来源

- `docs/superpowers/plans/[5]2026-06-29-ai-market-summary.md`

## 测试目标

验证 AI 市场摘要能力端到端可用：摘要任务配置、事件窗口读取、模板 provider、OpenAI-compatible mock provider、错误降级、免责声明、长度边界、Admin/通知可观测性，以及外部 AI 服务不可用时的系统稳定性。

## 前置条件

- 服务运行于 `http://127.0.0.1:18080`。
- 已有最近窗口内 market events 或可回放测试事件。
- AI provider 可使用本地 mock server；不使用真实生产 API key。
- 摘要消息最终可通过通知日志、服务日志或 mock sender 观察。

## 测试数据

| 名称 | 值 |
| --- | --- |
| Summary enabled | `CW_SUMMARY_ENABLED=true` |
| Provider template | `CW_SUMMARY_PROVIDER=template` |
| Provider openai-compatible | `CW_SUMMARY_PROVIDER=openai-compatible` |
| Disclaimer | `不构成投资建议` |
| Window | `900` 秒 |
| Max items | `50` |
| Mock model | `sit-summary-model` |

## 配置与启动

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 05-01 | 摘要关闭启动。 | `CW_SUMMARY_ENABLED=false` | App 启动；不会定时生成摘要；其他采集/规则/通知不受影响。 |
| 05-02 | 模板 provider 启动。 | `CW_SUMMARY_ENABLED=true`，provider=`template` | App 启动；无需外部 API key。 |
| 05-03 | openai-compatible mock 启动。 | provider=`openai-compatible`，API base 指向本地 mock | App 启动；摘要请求发往 mock。 |
| 05-04 | provider 未知。 | `CW_SUMMARY_PROVIDER=unknown` | 配置校验或运行日志给出受控错误；App 不 panic。 |
| 05-05 | API key 为空。 | openai-compatible + 空 key | 不调用真实外网；错误受控；不泄露 key。 |

## 模板摘要路径

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 05-06 | 准备窗口内事件。 | Binance/OKX 事件各若干条 | 事件可在 Admin Events 查询到。 |
| 05-07 | 触发摘要任务。 | 等待 interval 或调用测试 harness | 生成摘要文本。 |
| 05-08 | 检查摘要内容。 | template provider 输出 | 包含重点交易所/交易对/事件数量或告警概览；包含免责声明 `不构成投资建议`。 |
| 05-09 | 无事件窗口触发摘要。 | 清空或使用无事件窗口 | 生成“暂无显著异动”类受控摘要或跳过发送；不得返回空指针错误。 |
| 05-10 | max_items 边界。 | 准备超过 50 条事件 | 摘要只读取上限内数据；不一次性加载无界大结果。 |

## OpenAI-compatible mock 路径

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 05-11 | mock 返回成功摘要。 | HTTP 200，choices/message/content | 系统使用 mock 内容并追加或保留免责声明。 |
| 05-12 | mock 记录请求体。 | 检查 body | 请求包含 model、messages 或等价字段；不包含数据库 DSN、Bearer Token、session cookie。 |
| 05-13 | mock 返回 401。 | HTTP 401 | 摘要任务失败受控；记录错误；App 继续运行。 |
| 05-14 | mock 返回 429。 | HTTP 429 | 失败受控，不无限重试；下一轮仍可执行。 |
| 05-15 | mock 返回 500。 | HTTP 500 | 失败受控；不会影响 collector。 |
| 05-16 | mock 返回畸形 JSON。 | body=`{bad` | 解析错误受控；不产生错误摘要。 |
| 05-17 | mock 返回超长内容。 | 例如 20KB 文本 | 系统按实现约束处理；通知或日志不导致页面卡死；Admin 列表仍有界。 |

## 通知与可观测性

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 05-18 | 摘要发送到 Telegram/Webhook。 | mock sender 或沙箱通道 | 通知日志记录摘要发送结果；失败不影响后续摘要任务。 |
| 05-19 | 查询 Admin Notifications。 | `status=sent/failed` | 能看到摘要相关通知，列表有上限。 |
| 05-20 | 查询 Admin Trends。 | `/api/v1/admin/trends` | 通知计数与摘要通知结果一致或合理增加。 |
| 05-21 | 浏览器 Admin 刷新。 | 正确 Bearer | 页面无 JS error；长摘要不会撑坏列表布局。 |

## 安全与边界

| 编号 | 操作 | 输入 | 预期结果 |
| --- | --- | --- | --- |
| 05-22 | 检查日志。 | 服务日志 | 不打印真实 AI API key。 |
| 05-23 | 检查摘要内容。 | 输出文本 | 不出现“保证收益”“买入/卖出指令”等超出摘要范围内容；保留免责声明。 |
| 05-24 | 高频触发摘要。 | 缩短 interval 或连续调用 harness | 不并发堆积无界任务；内存和通知数量可控。 |
| 05-25 | 事件字段缺失。 | 缺 notional/title 的事件 | 摘要生成受控 fallback，不 panic。 |

## 通过标准

- 模板 provider 和 mock openai-compatible provider 至少各完成一次成功摘要。
- 401/429/500/超时/畸形 JSON 均为受控失败。
- 摘要读取窗口和 max_items 有边界。
- 摘要始终包含免责声明或明确非投资建议表达。
- 不泄露 API key、token、session、DSN。

## 证据留存

- mock AI server 请求记录。
- 摘要输出文本。
- Admin notification/trend 截图或 JSON。
- 服务日志中成功和失败任务片段。

## 清理事项

- 停止 mock AI server。
- 关闭摘要定时任务或恢复默认 interval。
