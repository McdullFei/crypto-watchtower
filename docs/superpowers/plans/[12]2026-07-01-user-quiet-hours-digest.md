# User Quiet Hours And Digest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users control noisy Telegram delivery with quiet hours and an optional bounded digest mode without changing operator/admin notification behavior.

**Architecture:** Keep real-time system alerts and admin APIs unchanged. Add user-scoped notification preference fields for quiet hours and digest mode, apply those preferences inside user fanout before Telegram sends, and surface controls in the Dashboard. Suppressed or digested deliveries must still leave bounded notification logs with `user_id`.

**Tech Stack:** Go 1.24, standard `net/http`, PostgreSQL 16.14 migrations/repositories, Redis 7.0.15 for bounded digest accumulation, embedded Dashboard UI, Docker Compose smoke.

---

## Acceptance Criteria

- [x] Logged-in users can configure Telegram quiet hours using session-protected APIs.
- [x] Quiet hours are evaluated with an explicit user timezone and do not affect operator/admin notifications.
- [x] User-rule matches during quiet hours skip Telegram sends and write notification logs with status `quiet_hours`.
- [x] Logged-in users can enable or disable digest mode for Telegram.
- [x] Digest mode accumulates bounded user alerts and emits a summarized Telegram message on schedule.
- [x] Digest accumulation is bounded by user, time window, and maximum item count.
- [x] Dashboard shows quiet-hours controls, digest controls, and current preference state.
- [x] Existing direct Telegram delivery, unbind, delivery toggle, and notification-log APIs keep their behavior.
- [x] Verification gate passes.

## Scope Boundary

- This plan includes user quiet hours, Telegram digest preference, bounded digest accumulation, Dashboard controls, and docs.
- This plan does not include email/SMS/mobile push, team preferences, paid entitlement changes, full retry orchestration, or AI-generated digest copy.

## File Structure

- Create/modify: `migrations/007_user_notification_preferences.sql` - quiet hours and digest preference fields.
- Modify: `internal/model/user.go` - preference fields.
- Modify: `internal/storage/user_repo.go` - read/update quiet-hours and digest preferences.
- Modify: `internal/user/service.go` - preference reads/writes for Dashboard.
- Modify: `internal/api/user.go` - session-protected preference API.
- Modify: `internal/api/user_test.go` - preference API authorization and scoping tests.
- Modify: `internal/rule/engine.go` - quiet-hours suppression and digest accumulation in user fanout.
- Modify: `internal/rule/engine_test.go` - quiet-hours and digest fanout tests.
- Modify: `internal/api/dashboardui/*` - Dashboard controls.
- Modify: `README.md`, `docs/用户手册.md`, `docs/plan/币圈异动监控平台总体开发计划.md` - document behavior and limits.

## Task 1: Preference Storage

**Files:**
- Create/modify: `migrations/007_user_notification_preferences.sql`
- Modify: `internal/model/user.go`
- Modify: `internal/storage/user_repo.go`
- Modify: `internal/storage/migration_test.go`
- Modify: `internal/integration/postgres_redis_test.go`

- [x] **Step 1: Write failing migration and repository tests**
- [x] **Step 2: Add quiet-hours and digest preference storage**
- [x] **Step 3: Implement bounded preference read/update methods**
- [x] **Step 4: Run storage and integration tests**

## Task 2: Preference API And Dashboard

**Files:**
- Modify: `internal/user/service.go`
- Modify: `internal/api/user.go`
- Modify: `internal/api/user_test.go`
- Modify: `internal/api/dashboardui/index.html`
- Modify: `internal/api/dashboardui/app.js`
- Modify: `internal/api/dashboardui/styles.css`

- [x] **Step 1: Write failing session-scoped preference API tests**
- [x] **Step 2: Implement preference read/update endpoints**
- [x] **Step 3: Render quiet-hours and digest controls in Dashboard**
- [x] **Step 4: Run API tests and `node --check`**

## Task 3: Quiet-Hours Fanout

**Files:**
- Modify: `internal/rule/engine.go`
- Modify: `internal/rule/engine_test.go`
- Modify: `internal/storage/alert_rule_repo.go` if preference joins are needed.

- [x] **Step 1: Write failing quiet-hours fanout tests**
- [x] **Step 2: Skip Telegram sends during quiet hours**
- [x] **Step 3: Record notification logs with status `quiet_hours`**
- [x] **Step 4: Run rule tests**

## Task 4: Digest Accumulation

**Files:**
- Modify: `internal/rule/engine.go`
- Modify: `internal/rule/engine_test.go`
- Create/modify: `internal/scheduler/user_digest.go` if a scheduler is needed.

- [x] **Step 1: Write failing bounded digest accumulation tests**
- [x] **Step 2: Accumulate digest items by user and window**
- [x] **Step 3: Emit digest Telegram messages on schedule**
- [x] **Step 4: Run rule and scheduler tests**

## Task 5: Docs And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`
- Modify: `docs/superpowers/plans/[12]2026-07-01-user-quiet-hours-digest.md`

- [x] **Step 1: Document quiet hours and digest behavior**
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

- 2026-07-01: Plan created after `[11]2026-07-01-user-notification-log-unbind.md` implementation. Scope is quiet-hours and digest controls only; new channels, paid entitlement changes, and full retry orchestration remain out of scope.
- 2026-07-01: Implementation work added migration `007_user_notification_preferences.sql`, user preference fields/repository update path, session-scoped `/api/v1/user/telegram/preferences`, Dashboard quiet-hours/digest controls, quiet-hours fanout logging `quiet_hours`, bounded digest queues, and a scheduler-driven digest flush path. RED checks observed before implementation: missing migration file and missing repository model/methods; preference API returned 404 before implementation.
- 2026-07-01: Verification passed with `node --check internal/api/dashboardui/app.js`; Docker Go 1.24 `go test ./...`; targeted real PostgreSQL/Redis integration `CW_INTEGRATION_TESTS=1 go test -tags integration ./internal/integration -run "UserNotificationPreferences|UserTelegramUnbind|UserDeliveryPreference|TelegramBindingRepositories|AuthRepositories" -v`; `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh`; `curl -fsS http://127.0.0.1:18080/dashboard`; and `git diff --check`.
