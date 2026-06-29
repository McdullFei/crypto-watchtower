# Dual Exchange Smoke And Phase 3 Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the current OKX/Admin work with dual-exchange verification, synchronized planning docs, and a clear Phase 3 entry point.

**Architecture:** Treat this as a verification and readiness slice, not a new product feature. Reuse the existing Go modular monolith, Docker Compose smoke script, Admin static UI, collector health status, and plan documents; do not introduce a new frontend framework or a second smoke harness unless the existing script cannot cover the matrix.

**Tech Stack:** Go 1.24, Docker Compose, PostgreSQL 16.14, Redis 7.0.15, Binance WebSocket/REST, OKX API v5 public WebSocket/REST, native `net/http` Admin UI, Node.js for static JavaScript checks.

---

## Current Progress

- [x] This third sub-plan file was created under `docs/superpowers/plans`.
- [x] Existing plan files were inspected: `[1]2026-05-20-crypto-watchtower-mvp-implementation.md` and `[2]2026-06-29-okx-exchange-integration.md`.
- [x] Current master plan status was inspected: Phase 1.5, Phase 2-A, and most OKX tasks are already checked off.
- [x] Current worktree verification has been fully rerun after this plan file was added.
- [x] Dual-exchange smoke matrix has been executed and recorded.
- [x] Master plan "current recommended next step" has been updated to this readiness slice.
- [x] Phase 3 first implementation slice has been selected after verification.

## File Structure

- Create: `docs/superpowers/plans/[3]2026-06-29-dual-exchange-smoke-and-phase3-readiness.md` - this third sub-plan and live progress tracker.
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md` - update the stale "current recommended next step" section to point at this readiness slice.
- Modify: `deployments/docker-compose.yml` - pass OKX environment variables into the app container so smoke runs can enable OKX.
- Optionally modify: `scripts/smoke-docker-compose.sh` - only if the current script cannot express Binance-only, OKX-only, and dual-exchange runs through environment variables.
- Optionally modify: `README.md` - only if smoke commands or Admin bilingual behavior need operator-facing instructions after verification.

## Task 1: Plan Setup And Baseline Audit

**Files:**
- Create: `docs/superpowers/plans/[3]2026-06-29-dual-exchange-smoke-and-phase3-readiness.md`
- Inspect: `docs/superpowers/plans`
- Inspect: `docs/plan/币圈异动监控平台总体开发计划.md`
- Inspect: `README.md`

- [x] **Step 1: Confirm this is the third sub-plan**

Run:

```bash
ls -1 docs/superpowers/plans
```

Expected: exactly these existing files before this plan is added:

```text
[1]2026-05-20-crypto-watchtower-mvp-implementation.md
[2]2026-06-29-okx-exchange-integration.md
```

- [x] **Step 2: Create this third sub-plan file**

Create:

```text
docs/superpowers/plans/[3]2026-06-29-dual-exchange-smoke-and-phase3-readiness.md
```

Expected: the plan contains checkbox progress and tasks for verification, dual-exchange smoke, master-plan sync, and Phase 3 readiness.

- [x] **Step 3: Inspect current phase status**

Run:

```bash
sed -n '107,230p' docs/plan/币圈异动监控平台总体开发计划.md
```

Expected: Phase 2-A is checked, OKX tasks are mostly checked, and "完成双交易所 smoke test" is still unchecked.

## Task 2: Current Worktree Verification Gate

**Files:**
- Verify: `internal/api/adminui/app.js`
- Verify: `internal/api/adminui/index.html`
- Verify: `internal/api/admin_test.go`
- Verify: complete Go module via Docker when allowed

- [x] **Step 1: Check Admin JavaScript syntax**

Run:

```bash
node --check internal/api/adminui/app.js
```

Expected: command exits 0 with no syntax errors.

- [x] **Step 2: Check Admin bilingual translation-key coverage**

Run:

```bash
node - <<'NODE'
const fs = require('fs');
const html = fs.readFileSync('internal/api/adminui/index.html', 'utf8');
const js = fs.readFileSync('internal/api/adminui/app.js', 'utf8');
const keys = [...html.matchAll(/data-i18n="([^"]+)"/g)].map((m) => m[1]);
const missing = keys.filter((key) => !js.includes(`"${key}":`));
if (missing.length) {
  console.error(`missing translations: ${missing.join(', ')}`);
  process.exit(1);
}
console.log(`checked ${keys.length} data-i18n entries`);
NODE
```

Expected: output includes `checked 33 data-i18n entries`.

- [x] **Step 3: Check patch whitespace**

Run:

```bash
git diff --check
```

Expected: command exits 0 with no whitespace errors.

- [x] **Step 4: Run Go tests through the project Docker toolchain**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
```

Expected: all Go packages pass. If Docker socket access is denied by the current sandbox, leave this step unchecked and record the denial in this plan.

## Task 3: Dual-Exchange Smoke Matrix

**Files:**
- Modify: `deployments/docker-compose.yml`
- Verify: `scripts/smoke-docker-compose.sh`
- Verify: `configs/config.example.yaml`
- Verify: `cmd/server/main.go`
- Verify: `internal/collector/okx_ws.go`
- Verify: `internal/api/adminui/index.html`

- [x] **Step 0: Pass OKX smoke variables into the app container**

Modify `deployments/docker-compose.yml` so the app service environment includes:

```yaml
CW_OKX_ENABLED: ${CW_OKX_ENABLED:-false}
CW_OKX_PUBLIC_WS_BASE_URL: ${CW_OKX_PUBLIC_WS_BASE_URL:-wss://ws.okx.com:8443/ws/v5/public}
CW_OKX_REST_BASE_URL: ${CW_OKX_REST_BASE_URL:-https://www.okx.com}
CW_OKX_SYMBOLS: ${CW_OKX_SYMBOLS:-BTCUSDT}
```

Expected: `APP_HTTP_PORT=18080 docker compose --env-file deployments/.env.local -f deployments/docker-compose.yml config` shows the OKX variables under `services.app.environment`.

- [x] **Step 1: Run Binance-only smoke**

Run:

```bash
CW_OKX_ENABLED=false APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

Expected: `/health` returns app, PostgreSQL, Redis, and Binance collector health; Admin page loads; protected rule write smoke succeeds; no OKX collector is required.

- [x] **Step 2: Run OKX-only smoke**

Run:

```bash
CW_BINANCE_ENABLED=false CW_OKX_ENABLED=true CW_OKX_SYMBOLS=BTCUSDT APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

Expected: HTTP service starts, OKX collector is present in `/health`, Admin exchange filter supports `okx`, and Binance collector absence does not fail startup. If `CW_BINANCE_ENABLED` does not exist yet, stop here and add the smallest config change needed before rerunning.

- [x] **Step 3: Run dual-exchange smoke**

Run:

```bash
CW_OKX_ENABLED=true CW_OKX_SYMBOLS=BTCUSDT APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

Expected: Binance and OKX collectors can be configured together, `/health` remains responsive, and Admin filters can request `exchange=binance` and `exchange=okx`.

- [x] **Step 4: Verify one collector failure does not break HTTP**

Run:

```bash
CW_OKX_ENABLED=true CW_OKX_PUBLIC_WS_BASE_URL=wss://127.0.0.1:1/ws CW_OKX_SYMBOLS=BTCUSDT APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

Expected: `/health` still responds, OKX status records an error, and the HTTP service plus non-OKX paths remain available.

## Task 4: Master Plan Synchronization

**Files:**
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`

- [x] **Step 1: Replace stale current-next-step text**

Modify section `## 4. 当前推荐下一步` so it says the current slice is:

```text
双交易所 smoke 验收与 Phase 3 前置准备
```

Expected: the section no longer says the next step is starting Phase 1.5.

- [x] **Step 2: Add link to this third sub-plan**

Add this exact path in the section:

```text
docs/superpowers/plans/[3]2026-06-29-dual-exchange-smoke-and-phase3-readiness.md
```

Expected: the master plan points future workers to this file as the active execution plan.

- [x] **Step 3: Mark completed prerequisites**

Ensure the section records that these are already done:

```text
Phase 1.5 收口
Phase 2-A 运营后台最小可视化闭环
OKX 第二交易所接入主体
运营后台中英双语支持
```

Expected: the remaining next action is dual-exchange smoke, not reopening completed phases.

## Task 5: Phase 3 Entry Decision

**Files:**
- Inspect: `docs/plan/币圈异动监控平台总体开发计划.md`
- Optionally create later: a fourth focused plan for the selected Phase 3 slice

- [x] **Step 1: Confirm verification evidence is strong enough**

Run:

```bash
git diff --check
```

Run, when Docker is available:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
```

Run, when Docker is available:

```bash
CW_OKX_ENABLED=true CW_OKX_SYMBOLS=BTCUSDT APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

Expected: all three verification gates pass before selecting a Phase 3 implementation task.

- [x] **Step 2: Select Phase 3 first slice**

Default selection after verification:

```text
Discord / Webhook 通知
```

Reason: it reuses the existing notifier boundary and `notification_logs`, has lower blast radius than user accounts or SaaS billing, and gives an immediate operator-facing product improvement.

- [x] **Step 3: Create the Phase 3 implementation plan only after this plan is verified**

```text
docs/superpowers/plans/[4]2026-06-29-discord-webhook-notifier.md
```

Expected: do not implement this fourth plan until the user explicitly starts Phase 3 work.

## Execution Notes

- 2026-06-29: Started this plan as the third sub-plan file.
- 2026-06-29: `node --check internal/api/adminui/app.js` passed.
- 2026-06-29: Admin bilingual translation-key coverage passed with `checked 33 data-i18n entries`.
- 2026-06-29: `git diff --check` passed.
- 2026-06-29: `docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'` passed.
- 2026-06-29: `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` passed.
- 2026-06-29: `CW_OKX_ENABLED=true CW_OKX_SYMBOLS=BTCUSDT APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` passed; `/health` showed `binance-spot`, `binance-futures`, and `okx-public`.
- 2026-06-29: `CW_OKX_ENABLED=true CW_OKX_PUBLIC_WS_BASE_URL=wss://127.0.0.1:1/ws CW_OKX_SYMBOLS=BTCUSDT APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` passed; `/health` stayed `up` and OKX recorded `connect: connection refused`.
- 2026-06-29: Added `binance.enabled` / `CW_BINANCE_ENABLED` so OKX-only smoke can disable Binance collectors and funding fetcher.
- 2026-06-29: `CW_BINANCE_ENABLED=false CW_OKX_ENABLED=true CW_OKX_SYMBOLS=BTCUSDT APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh` passed; `/health` showed only `okx-public`.
- 2026-06-29: Browser smoke opened `/admin` through Playwright and captured a page snapshot, verifying Chinese UI, language selector, exchange selector with OKX, and the main Admin panels. Playwright interactive commands (`fill`, `select`, `run-code`, later `snapshot`) hung and were aborted; Admin data APIs with Bearer token and `exchange=okx` filters were verified with `curl`.
- 2026-06-29: Created Phase 3 first-slice plan at `docs/superpowers/plans/[4]2026-06-29-discord-webhook-notifier.md`; implementation has not started.
