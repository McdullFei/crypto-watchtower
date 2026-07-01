# User Rule Window And Delivery Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete user-rule delivery polish after Telegram fanout by supporting user-level window rules, adding explicit delivery controls, and surfacing delivery state in the Dashboard without changing operator/admin alerting behavior.

**Architecture:** Keep system rules and operator notifications unchanged. Extend the user fanout path with bounded per-user/rule window state, add a small persisted delivery preference layer, and make Dashboard controls session-scoped. All event scans and rule lookups must stay bounded.

**Tech Stack:** Go 1.24, standard `net/http`, PostgreSQL 16.14 migrations/repositories, Redis 7.0.15 for bounded window/dedupe state, embedded Dashboard UI, Docker Compose smoke.

---

## Acceptance Criteria

- [x] User-scoped `large_trade_window` rules evaluate with bounded Redis state and do not reuse global system rule keys.
- [x] User rule delivery can be explicitly enabled or disabled for Telegram without deleting the binding.
- [x] Disabled user delivery skips Telegram sends and records an intentional skipped/disabled status where notification logs are written.
- [x] Dashboard shows Telegram binding status, delivery enabled state, and recent delivery status without exposing raw chat ids.
- [x] User APIs remain session-protected and do not accept another user's rule, binding, or delivery preference IDs.
- [x] Operator Bearer Token APIs and system rule notifications keep their current behavior.
- [x] Query limits and fanout batch sizes remain bounded.
- [x] Verification gate passes.

## Scope Boundary

- This plan includes user-level window rule evaluation, Telegram delivery enable/disable controls, Dashboard delivery-state UX, and documentation.
- This plan does not include email/SMS/mobile push, team notification routing, billing entitlements beyond existing plan limits, quiet hours, or notification digest scheduling.

## File Structure

- Modify: `internal/rule/engine.go` - user-level `large_trade_window` evaluation and Redis key isolation.
- Modify: `internal/rule/engine_test.go` - user window fanout and disabled-delivery tests.
- Create/modify: `migrations/006_user_delivery_preferences.sql` - delivery preference table or user preference column.
- Modify: `internal/model/user.go` - delivery preference model if needed.
- Modify: `internal/storage/user_repo.go` - read/update user delivery preference.
- Modify: `internal/api/user.go` - session-protected delivery preference endpoints.
- Modify: `internal/api/user_test.go` - API authorization and preference tests.
- Modify: `internal/api/dashboardui/*` - Dashboard delivery controls and recent state.
- Modify: `README.md`, `docs/用户手册.md`, `docs/plan/币圈异动监控平台总体开发计划.md` - document behavior and limits.

## Task 1: User Window Rule Evaluation

**Files:**
- Modify: `internal/rule/engine.go`
- Modify: `internal/rule/engine_test.go`

- [x] **Step 1: Write failing user `large_trade_window` fanout tests**
- [x] **Step 2: Add isolated bounded Redis keys for user window state**
- [x] **Step 3: Preserve existing system window rule behavior**
- [x] **Step 4: Run rule tests**

## Task 2: Delivery Preference Storage

**Files:**
- Create/modify: `migrations/006_user_delivery_preferences.sql`
- Modify: `internal/model/user.go`
- Modify: `internal/storage/user_repo.go`
- Modify: `internal/storage/migration_test.go`
- Modify: `internal/integration/postgres_redis_test.go`

- [x] **Step 1: Write failing migration/repository tests**
- [x] **Step 2: Add Telegram delivery preference storage**
- [x] **Step 3: Implement bounded read/update repository methods**
- [x] **Step 4: Run storage and integration tests**

## Task 3: API And Dashboard Controls

**Files:**
- Modify: `internal/api/user.go`
- Modify: `internal/api/user_test.go`
- Modify: `internal/api/dashboardui/index.html`
- Modify: `internal/api/dashboardui/app.js`
- Modify: `internal/api/dashboardui/styles.css`

- [x] **Step 1: Write failing session-protected preference API tests**
- [x] **Step 2: Implement delivery preference read/update endpoints**
- [x] **Step 3: Add Dashboard toggle and delivery-state rendering**
- [x] **Step 4: Run API tests and `node --check`**

## Task 4: Fanout Delivery Control

**Files:**
- Modify: `internal/rule/engine.go`
- Modify: `internal/rule/engine_test.go`
- Modify: `internal/storage/alert_rule_repo.go` if preference joins are needed.

- [x] **Step 1: Write failing disabled-delivery fanout tests**
- [x] **Step 2: Skip Telegram sends when user delivery is disabled**
- [x] **Step 3: Record bounded notification log status for disabled delivery**
- [x] **Step 4: Run rule and storage tests**

## Task 5: Docs And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`
- Modify: `docs/superpowers/plans/[10]2026-07-01-user-rule-window-delivery-controls.md`

- [x] **Step 1: Document user window rules and delivery controls**
- [x] **Step 2: Update master plan status**
- [x] **Step 3: Run verification gate**

Run:

```bash
node --check internal/api/dashboardui/app.js
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
curl -fsS http://127.0.0.1:18080/dashboard
git diff --check
```

Expected: PASS.

## Execution Notes

- 2026-07-01: Plan created after `[9]2026-07-01-user-telegram-fanout.md` completed. Scope is delivery polish and user window-rule completeness only; quiet hours, digest scheduling, and new notification channels remain out of scope.
- 2026-07-01: Implementation completed. RED/GREEN coverage included user `large_trade_window` fanout, Telegram delivery preference migration/repository, session-scoped preference API, Dashboard controls, disabled-delivery fanout logging, and recent delivery status. Verification gate passed with `node --check internal/api/dashboardui/app.js`; Docker Go 1.24 `go test ./...`; Docker Compose smoke; integration `go test -tags integration ./internal/integration -run "UserDeliveryPreference|TelegramBindingRepositories|AuthRepositories" -v`; Dashboard curl; and `git diff --check`.
