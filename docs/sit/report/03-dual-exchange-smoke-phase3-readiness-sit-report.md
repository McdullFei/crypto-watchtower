# SIT-03 双交易所 Smoke 与 Phase 3 准备度全量 E2E 测试报告

## 基本信息

- 测试日期：2026-07-02
- 测试环境：本地 Docker Compose，`http://127.0.0.1:18080`
- 测试用例：`docs/sit/case/03-dual-exchange-smoke-phase3-readiness-sit.md`
- Admin Token：`change-me`
- 测试技能：Build Web Apps:frontend-testing-debugging、superpowers:test-driven-development、superpowers:systematic-debugging、Playwright CLI
- Browser 路径：Browser plugin 未在当前会话暴露，按 Build Web Apps 技能要求使用 Playwright CLI fallback

## 总体结论

SIT-03 最终通过。

- 03-01 到 03-28 均已验证通过。
- 本轮发现 2 个测试 bug，均已按 TDD 流程补充自动化测试、修复并复测通过。
- 默认 smoke、完整 Go 回归、Admin UI、Dashboard UI、故障注入和 Phase 3 用户 API 均已覆盖。
- 测试结束后已恢复默认 collector 配置，并禁用本轮低阈值个人测试规则。

## 测试结果

| 编号 | 结果 | 说明 |
| --- | --- | --- |
| 03-01 | 通过 | `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` 通过；覆盖 `/health`、读接口、写接口鉴权。 |
| 03-02 | 通过 | Binance-only 模式 `/health` 仅包含 `binance-spot`、`binance-futures`，Admin 静态页面可加载。 |
| 03-03 | 通过 | OKX-only 模式 `/health` 包含 `okx-public`；外网 EOF 为受控 collector 状态，Admin API 返回 `code=0`。 |
| 03-04 | 通过 | Dual 模式同时包含 Binance 与 OKX collector；`exchange=binance` 与 `exchange=okx` 查询均返回 `code=0`。 |
| 03-05 | 通过 | No collector 模式 `/health.collectors=[]`；用户注册、profile、用户规则 API 可用。 |
| 03-06 | 通过 | Playwright 打开 `/admin`，输入 `change-me` 后概览、运行状态、趋势、规则、告警、事件、通知面板均渲染。 |
| 03-07 | 通过 | Admin 在 OKX/Binance/全部间切换，并验证 limit 1/200/8；请求有界，页面未卡死，列表无 exchange 错乱。 |
| 03-08 | 通过 | Playwright 打开 `/dashboard`，注册 `sit-phase3-ready-ui-20260702@example.test` / `Strong1!`，登录态建立，用户面板加载。 |
| 03-09 | 通过 | Dashboard 创建 Binance 与 OKX 各一条 `BTCUSDT` 个人规则；列表同时展示两条，互不覆盖。 |
| 03-10 | 通过 | Dashboard 连续点击 Reload 5 次后状态回到 `Loaded.`，规则列表仍稳定展示。 |
| 03-11 | 通过 | OKX WS 指向 `ws://127.0.0.1:1` 后 health 显示 OKX `connection refused`；Binance replay 事件仍成功。 |
| 03-12 | 通过 | 修复 compose 后，Binance spot/futures WS 指向 `ws://127.0.0.1:1` 均显示 `connection refused`；OKX replay 事件仍成功。 |
| 03-13 | 通过 | 停止 PostgreSQL 后 `/health.dependencies.postgres.status=error`；Admin 列表接口受控返回 HTTP 500；app 容器仍运行且日志无 panic。 |
| 03-14 | 通过 | 恢复 PostgreSQL 后 `/health` 恢复 `postgres=ok`，Admin events 列表恢复 HTTP 200。 |
| 03-15 | 通过 | 停止 Redis 后 `/health.dependencies.redis.status=error`；`/dashboard`、`/dashboard/app.js`、`/dashboard/styles.css` 均返回 200。 |
| 03-16 | 通过 | 快速停止并启动 app 容器后 `/health`、`/admin`、`/dashboard` 均恢复可访问。 |
| 03-17 | 通过 | Admin overview/trends/rules/alerts/events/notifications 无 token 均返回 401；`/health` 无需 token。 |
| 03-18 | 通过 | Admin overview/trends/rules/alerts/events/notifications 使用 `Bearer bad` 均返回 401。 |
| 03-19 | 通过 | Admin overview/trends/rules/alerts/events/notifications 使用 `Bearer change-me` 均返回 HTTP 200 且 `code=0`。 |
| 03-20 | 通过 | `exchange=nope` 返回 HTTP 200、`code=0`、`data=[]`，无 500。 |
| 03-21 | 通过 | `limit=abc` 使用受控默认上限，返回 HTTP 200 且未返回无界大结果。 |
| 03-22 | 通过 | `user_id=-1`、`user_id=abc` 均受控返回 HTTP 200；用户 API 通过 session 隔离，不接受 query user_id 越权。 |
| 03-23 | 通过 | 用户注册、登录、profile、logout 均可用；logout 后 profile 返回 401。 |
| 03-24 | 通过 | `/api/v1/user/rules` 按 session 隔离；第二个用户无规则时修复后返回 `data=[]`。 |
| 03-25 | 通过 | `/api/v1/user/telegram/binding-token` 未登录返回 401，登录后生成短期 token。 |
| 03-26 | 通过 | `/api/v1/user/telegram/delivery` 未登录返回 401，登录后可切换为 disabled 并返回更新后的 profile。 |
| 03-27 | 通过 | `/api/v1/user/notifications?limit=5` 未登录返回 401，登录后返回有界列表。 |
| 03-28 | 通过 | `/dashboard`、`/dashboard/app.js`、`/dashboard/styles.css` 均返回 200。 |

## Bug 与修复复测

### Bug 1：Binance endpoint 覆盖项未透传到 Docker Compose

- 现象：03-12 中设置 `CW_BINANCE_SPOT_WS_BASE_URL=ws://127.0.0.1:1`、`CW_BINANCE_FUTURES_WS_BASE_URL=ws://127.0.0.1:1` 后，health 仍显示 futures collector 正常连接，故障注入没有进入容器。
- 原因：`deployments/docker-compose.yml` 未将 Binance endpoint 覆盖项写入 app environment。
- TDD 复现：新增 `deployments/docker_compose_test.go` 中的 `TestDockerComposePassesBinanceEndpointOverrides`。
- 修复：在 `deployments/docker-compose.yml` app environment 增加 `CW_BINANCE_SPOT_WS_BASE_URL`、`CW_BINANCE_FUTURES_WS_BASE_URL`、`CW_BINANCE_FUTURES_REST_BASE_URL` 透传。
- 复测结果：`go test ./deployments -run TestDockerComposePassesBinanceEndpointOverrides -v` 通过；重新注入 Binance WS 故障后，spot/futures 均显示 `connection refused`，OKX replay 仍返回 `event replayed`。

### Bug 2：空用户规则列表返回 `data:null`

- 现象：03-05/03-24 中无规则用户调用 `/api/v1/user/rules` 返回 `{"data":null}`，列表响应形态不稳定。
- 原因：用户规则 handler 直接序列化 nil slice。
- TDD 复现：新增 `internal/api/user_test.go` 中的 `TestUserRulesListReturnsEmptyArray`。
- 修复：`internal/api/user.go` 的 `handleUserRulesList` 使用已有 `nonNilSlice` 输出空数组。
- 复测结果：`go test ./internal/api -run TestUserRulesListReturnsEmptyArray -v` 通过；新用户接口复测返回 `{"code":0,"data":[],"message":"ok"}`。

## 关键复测命令

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

复测结果：

- `go test ./...` 全部通过。
- 默认 Docker Compose smoke 通过。
- 最终 `/health`：默认配置下仅 Binance collector，PostgreSQL/Redis 均 `ok`。

## 浏览器验证

- Admin flow：`/admin` -> 输入 `change-me` -> 切换 exchange OKX/Binance/全部 -> limit 1/200/8 -> 列表稳定。
- Dashboard flow：`/dashboard` -> 注册 `sit-phase3-ready-ui-20260702@example.test` -> 创建 Binance/OKX 两条同 symbol 规则 -> Reload 5 次 -> 页面保持 `Loaded.`。
- Console 说明：未输入 token/未登录前存在预期的 401 resource console 记录，以及 favicon 404；完成认证后的目标交互未观察到未处理 JS runtime 异常。

## 清理状态

- 已禁用本轮 SIT-03 创建的 4 条低阈值个人测试规则：
  - `sit-phase3-ready-api-20260702@example.test`：Binance `1234`、OKX `2345`
  - `sit-phase3-ready-ui-20260702@example.test`：Binance `1111`、OKX `2222`
- 已执行默认 smoke，collector 配置恢复为默认 Binance-only。
- 已清理 Playwright 临时目录 `.playwright-cli/`。
