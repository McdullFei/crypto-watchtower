# SIT-05 AI 市场摘要全量 E2E 测试报告

## 基本信息

- 测试日期：2026-07-03
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 测试用例：`docs/sit/case/05-ai-market-summary-sit.md`
- Admin Token：`change-me`
- 测试技能：Build Web Apps frontend-testing-debugging、superpowers:test-driven-development
- 浏览器验证：Browser plugin not available，按 Build Web Apps 技能要求使用 Playwright CLI fallback

## 总体结论

SIT-05 最终通过。

- 05-01 至 05-25 均完成验证并达到预期。
- 测试中发现 2 个 bug，均按 TDD 流程先补失败测试，再修复并复测通过。
- Template provider 与 OpenAI-compatible mock provider 均完成 generated 摘要。
- 401、429、500、timeout、畸形 JSON 均落为受控 failed 摘要，App 保持运行且后续轮次可继续生成。
- 摘要内容保留 `不构成投资建议`，未出现保证收益或买入/卖出指令。
- 服务日志未出现 `sit-summary-key`、`postgres://`、`cw_session`。

## Bug 与修复复测

| 编号 | 问题 | TDD 失败用例 | 修复 | 复测结果 |
| --- | --- | --- | --- | --- |
| BUG-01 | `CW_SUMMARY_PROVIDER=unknown` 会被静默当作 template 运行，未给出受控配置错误。 | `TestValidateRejectsUnknownSummaryProvider` 初次失败：未知 provider 未返回错误。 | `Config.Validate` 增加 provider 白名单，仅允许 `template` 与 `openai_compatible`。 | 单测通过；E2E 日志返回 `summary.provider must be template or openai_compatible`，mock 请求 0 次。 |
| BUG-02 | 生成成功的摘要只写入 `market_summaries`，不会进入通知 sender 和 notification logs，Admin 侧不可观测。 | `TestServiceSendsGeneratedSummaryToNotificationSenders` 初次编译失败：`Service` 无 `Senders`/`Notifications` 能力。 | Summary service 增加可选 `NamedSender` 与 `NotificationLogStore`，生成成功后以 `market_summary` alert 复用 Telegram/Webhook sender 并写通知日志；server wiring 复用现有通知 sender。 | 单测通过；E2E 中 template/openai-compatible 摘要均产生 `telegram/default/sent` 通知日志。 |

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 05-01 | 通过 | `CW_SUMMARY_ENABLED=false` 启动成功，等待后 `market_summaries` 计数保持不变，其他健康检查正常。 |
| 05-02 | 通过 | `CW_SUMMARY_ENABLED=true`、provider=`template` 启动成功，无需外部 API key。 |
| 05-03 | 通过 | provider=`openai_compatible`、API base 指向本地 mock 后启动成功，mock 收到 `/v1/chat/completions` 请求。 |
| 05-04 | 通过 | provider=`unknown` 被配置校验拦截，日志给出明确错误，App 无 panic。 |
| 05-05 | 通过 | openai-compatible 空 API key 被配置校验拦截，mock 请求数为 0。 |
| 05-06 | 通过 | 创建 `SIT05TEMPLATEUSDT` 低阈值规则并 replay 事件，Admin Events 可查询到窗口内事件。 |
| 05-07 | 通过 | 通过 `CW_SUMMARY_INTERVAL_SEC=1` 触发摘要任务，摘要写入 `market_summaries`。 |
| 05-08 | 通过 | Template 摘要包含 `SIT05TEMPLATEUSDT`、`alerts=1`、`events=1`、规则/事件摘要和 `不构成投资建议`。 |
| 05-09 | 通过 | `CW_SUMMARY_WINDOW_SEC=1` 且无新事件时生成受控摘要：`alerts=0, events=0, funding=0`，包含免责声明。 |
| 05-10 | 通过 | 插入 55 条事件后最新摘要显示 `events=51`，实际样本事件行数为 50，读取有边界。 |
| 05-11 | 通过 | mock 返回 200 后保存 provider 内容，并自动追加或保留 `不构成投资建议`。 |
| 05-12 | 通过 | mock 请求体包含 `model=sit-summary-model` 和 `messages`；body 不含 `postgres://`、Bearer、session、`sit-summary-key`。 |
| 05-13 | 通过 | mock 返回 401 时摘要记录为 `failed`，错误为 `openai compatible summary request failed: status 401`。 |
| 05-14 | 通过 | mock 返回 429 时摘要记录为 `failed`；恢复 200 后下一轮可 generated。 |
| 05-15 | 通过 | mock 返回 500 时摘要记录为 `failed`，collector 和 HTTP 服务保持运行。 |
| 05-16 | 通过 | mock 返回 `{bad` 时摘要记录为 `failed`，错误为 JSON 解析摘要。 |
| 05-17 | 通过 | mock 返回约 12KB 内容时摘要 generated，免责声明存在，Admin notifications 仍按 limit 有界返回。 |
| 05-18 | 通过 | 摘要通过 Telegram local-sandbox sender 发送，notification logs 记录 `sent`；失败 provider 不影响后续摘要轮次。 |
| 05-19 | 通过 | Admin Notifications 查询到摘要相关 `telegram/default/sent` 记录，列表按 limit 返回。 |
| 05-20 | 通过 | Admin Trends 中 `sent` 计数随摘要通知合理增加。 |
| 05-21 | 通过 | Playwright CLI 打开 `/admin`，输入 token 后刷新成功，Notifications 表展示 sent 记录；长摘要未撑坏列表布局。 |
| 05-22 | 通过 | 服务日志扫描未出现真实 AI API key、session、DSN。 |
| 05-23 | 通过 | 摘要输出未包含“保证收益”等承诺，也未包含买入/卖出指令；保留免责声明。 |
| 05-24 | 通过 | `interval=1s` 高频触发期间 API `limit=5` 有界返回，App 健康检查为 `up`，未出现无界列表输出。 |
| 05-25 | 通过 | 插入 notional 为 0、side 为空的事件后，下一轮摘要 generated，无 panic。 |

## 关键证据

- Template 摘要示例：`alerts=1, events=1, funding=0`，包含 `SIT05TEMPLATEUSDT` 和 `不构成投资建议`。
- 无事件窗口摘要：`alerts=0, events=0, funding=0`。
- max_items 边界：插入 55 条事件，摘要样本事件行数为 50。
- OpenAI-compatible 成功请求：mock 记录 12 次请求，首个 body 中 `model=sit-summary-model`，body 敏感词命中为 false。
- OpenAI-compatible 恢复路径：timeout 后切回 200，最新摘要状态为 `generated`。
- 长内容路径：最新摘要 `length(content)=12009`，免责声明位置存在，Admin notifications 返回 `count=5`。
- 最终近 2 小时摘要统计：`openai_compatible/generated=387`、`openai_compatible/failed=232`、`template/generated=1228`。
- Admin UI：页面 title 为 `CryptoWatchtower Admin`，输入 `change-me` 后显示“后台数据已加载”，Notifications 表展示 `telegram · sent`。

## 执行命令摘要

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/config ./internal/summary ./cmd/server ./internal/api ./internal/admin -v'
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
APP_HTTP_PORT=18080 CW_SUMMARY_ENABLED=true CW_SUMMARY_PROVIDER=template CW_SUMMARY_INTERVAL_SEC=1 docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml up -d app
APP_HTTP_PORT=18080 CW_SUMMARY_ENABLED=true CW_SUMMARY_PROVIDER=openai_compatible CW_SUMMARY_API_BASE_URL=http://host.docker.internal:19191/v1 CW_SUMMARY_API_KEY=sit-summary-key CW_SUMMARY_MODEL=sit-summary-model docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml up -d app
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

## 清理结果

- 已禁用本次写入的低阈值规则 `SIT05TEMPLATEUSDT`。
- 已执行默认 smoke，App 恢复为默认本地配置。
- 已停止本地 AI mock server。
- 已清理 Playwright CLI 临时目录。
