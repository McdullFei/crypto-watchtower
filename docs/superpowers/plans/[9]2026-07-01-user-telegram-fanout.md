# User Telegram Fanout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the first user-level notification loop: a logged-in user can bind a Telegram chat to the account, personal rules can generate user-owned alerts, and matched alerts are delivered to that user's bound Telegram target with notification logs.

**Architecture:** Keep operator/admin alerting and system rules unchanged. Add a user binding-token flow between `/dashboard` and Telegram Bot polling, then evaluate bounded user rules separately from global system rules. User fanout writes `notification_logs.user_id` for each delivery attempt and skips disabled or unbound users.

**Tech Stack:** Go 1.24, standard `net/http`, PostgreSQL 16.14 migrations/repositories, Redis dedupe/rate-limit hooks, existing Telegram polling/notifier, embedded Dashboard UI, Docker Compose smoke.

---

## Acceptance Criteria

- [x] Logged-in users can request a short-lived Telegram binding token from `/api/v1/user/telegram/binding-token`.
- [x] Telegram `/start <binding_token>` binds the chat id to the matching account and consumes the token once.
- [x] `/api/v1/user/profile` reflects the masked Telegram binding after successful bind.
- [x] User-scoped rules are evaluated independently from system rules and do not mutate the global system rule engine.
- [x] A user-scoped rule match creates a user-owned alert or delivery record with `user_id`.
- [x] Bound users receive Telegram notifications for their own matched user rules.
- [x] Disabled users and unbound users are skipped without panics or unbounded retries.
- [x] Notification delivery attempts write bounded `notification_logs` rows with `user_id`, `channel`, `target`, `status`, and `error_message`.
- [x] Dashboard shows Telegram binding instructions and current binding state without exposing raw chat ids.
- [x] Operator Bearer Token APIs keep their current behavior.
- [x] Payment billing, organization/team accounts, OAuth, and multi-channel notification preference UI remain outside this plan.
- [x] Verification gate passes.

## Scope Boundary

- This plan includes Telegram account binding, user-rule evaluation, user Telegram fanout, notification logs, Dashboard binding UX, and documentation.
- This plan does not include paid billing, invoices, teams, OAuth, email/SMS delivery, mobile push, per-rule quiet hours, or self-service unbind history.
- User-rule evaluation must stay memory-bounded: load and evaluate rules in bounded batches or indexed lookups, not by pulling unbounded user/rule sets into memory on every event.

## File Structure

- Create: `migrations/005_user_telegram_binding.sql` - binding token table and indexes.
- Modify: `internal/model/user.go` - binding token model if needed.
- Create: `internal/user/telegram_binding.go` - binding token service.
- Create: `internal/user/telegram_binding_test.go` - binding token service tests.
- Modify: `internal/storage/user_repo.go` - bind Telegram chat id to account.
- Create: `internal/storage/telegram_binding_repo.go` - persisted binding tokens.
- Modify: `internal/storage/repositories.go` - expose binding repository.
- Modify: `internal/notifier/telegram_poller.go` - support `/start <binding_token>`.
- Modify: `internal/rule/service.go` - expose bounded user-rule listing for fanout.
- Modify: `internal/rule/pipeline.go` - evaluate user rules and deliver user notifications.
- Modify: `internal/storage/alert_repo.go` and `internal/storage/notification_log_repo.go` - persist user-owned delivery context where missing.
- Modify: `internal/api/user.go` - add binding-token endpoint.
- Modify: `internal/api/user_test.go` - session-protected binding-token API tests.
- Modify: `cmd/server/main.go` - wire binding service and user fanout dependencies.
- Modify: `internal/api/dashboardui/*` - show binding instructions and refresh state.
- Modify: `README.md`, `docs/用户手册.md`, `docs/plan/币圈异动监控平台总体开发计划.md` - document user Telegram fanout.

## Task 1: Binding Schema And Repository

**Files:**
- Create: `migrations/005_user_telegram_binding.sql`
- Modify: `internal/model/user.go`
- Create: `internal/storage/telegram_binding_repo.go`
- Modify: `internal/storage/user_repo.go`
- Modify: `internal/storage/repositories.go`
- Test: `internal/storage/migration_test.go`, `internal/integration/postgres_redis_test.go`

- [x] **Step 1: Write failing migration/repository tests**
- [x] **Step 2: Add binding token migration and model**
- [x] **Step 3: Implement binding token repository and account chat binding**
- [x] **Step 4: Run storage and integration tests**

## Task 2: User Binding Service

**Files:**
- Create: `internal/user/telegram_binding.go`
- Create: `internal/user/telegram_binding_test.go`

- [x] **Step 1: Write failing binding service tests**
- [x] **Step 2: Implement token generation, expiry, lookup, and one-time consume**
- [x] **Step 3: Run user binding tests**

## Task 3: Binding API And Dashboard UX

**Files:**
- Modify: `internal/api/user.go`
- Modify: `internal/api/user_test.go`
- Modify: `internal/api/dashboardui/index.html`
- Modify: `internal/api/dashboardui/app.js`
- Modify: `internal/api/dashboardui/styles.css`

- [x] **Step 1: Write failing session-protected binding-token API tests**
- [x] **Step 2: Implement `/api/v1/user/telegram/binding-token`**
- [x] **Step 3: Add Dashboard binding instructions and refresh behavior**
- [x] **Step 4: Run API tests and `node --check`**

## Task 4: Telegram Bot Binding Command

**Files:**
- Modify: `internal/notifier/telegram_poller.go`
- Modify: `internal/notifier/telegram_poller_test.go`
- Modify: `cmd/server/main.go`

- [x] **Step 1: Write failing `/start <binding_token>` poller tests**
- [x] **Step 2: Wire binding service into Telegram poller**
- [x] **Step 3: Consume token and bind chat id to account**
- [x] **Step 4: Run notifier tests**

## Task 5: User Rule Evaluation And Fanout

**Files:**
- Modify: `internal/rule/service.go`
- Modify: `internal/rule/pipeline.go`
- Modify: `internal/rule/service_test.go`
- Modify: `internal/storage/alert_rule_repo.go`
- Modify: `internal/storage/notification_log_repo.go`

- [x] **Step 1: Write failing user-rule fanout tests**
- [x] **Step 2: Add bounded user-rule lookup/evaluation path**
- [x] **Step 3: Deliver matched alerts only to bound active users**
- [x] **Step 4: Persist user delivery logs and skip disabled/unbound users**
- [x] **Step 5: Run rule and storage tests**

## Task 6: Docs And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`
- Modify: `docs/superpowers/plans/[9]2026-07-01-user-telegram-fanout.md`

- [x] **Step 1: Document binding and user fanout behavior**
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

- 2026-07-01: Plan created after `[8]2026-06-30-saas-auth-subscription.md` completed. Scope is user Telegram binding and fanout only; billing remains out of scope.
- 2026-07-01: Implementation reached verification gate. `node --check internal/api/dashboardui/app.js` and `git diff --check` passed. Docker-based Go/full integration/smoke verification is blocked by platform usage limit until 13:33 CST, so final acceptance criteria, Task 1 Step 4, Task 6 Step 2/3, and plan[10] creation remain intentionally unchecked.
- 2026-07-01: Verification gate passed after Docker access resumed: `node --check internal/api/dashboardui/app.js`; Docker Go 1.24 `gofmt` plus `go test ./...`; `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh`; integration `go test -tags integration ./internal/integration -run "TelegramBindingRepositories|AuthRepositories" -v`; `curl -fsS http://127.0.0.1:18080/dashboard`; and `git diff --check`.
