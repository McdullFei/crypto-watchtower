# Phase 3 User Notification Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the Phase 3 user notification slice by proving quiet-hours, digest, delivery toggle, unbind, and user notification logs work together without regressing operator/admin notifications.

**Architecture:** This is a closeout and acceptance plan, not a feature expansion. It rechecks user-facing notification APIs, Dashboard controls, rule fanout behavior, scheduler wiring, docs consistency, and smoke/runtime gates against the current implementation. Any fix found during verification must stay scoped to the failing acceptance point.

**Tech Stack:** Go 1.24, PostgreSQL 16.14, Redis 7.0.15, Docker Compose, standard `net/http`, embedded Dashboard UI, shell verification commands.

---

## Acceptance Criteria

- [x] Plan `[12]` is fully checked and references current verification evidence.
- [x] User Telegram delivery states are covered: direct `sent`, `disabled`, `quiet_hours`, and `digested`.
- [x] Digest queues are bounded by user, time window, and item count, and scheduler wiring calls due flushes.
- [x] User preference APIs are session-scoped and do not honor query/body `user_id`.
- [x] Dashboard exposes delivery, unbind, quiet-hours, digest, alerts, and notification logs controls without JS syntax errors.
- [x] Docs explain the user notification behavior in README, user manual, and master plan.
- [x] Final acceptance suite passes or any blocked command is recorded with exact blocker.

## Scope Boundary

- This plan includes closeout fixes, documentation consistency, and final test/smoke acceptance.
- This plan does not add new notification channels, billing, team preferences, retry orchestration, or AI digest copy.

## File Structure

- Modify: `docs/superpowers/plans/[12]2026-07-01-user-quiet-hours-digest.md` - check completed plan items and record evidence.
- Modify: `docs/superpowers/plans/[13]2026-07-01-phase3-user-notification-closeout.md` - check closeout tasks and record evidence.
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md` - move the next recommended step after user notification closeout.
- Modify only if verification finds a scoped defect: files under `internal/api`, `internal/rule`, `internal/scheduler`, `internal/storage`, or `internal/api/dashboardui`.

## Task 1: Plan 12 Completion Audit

**Files:**
- Modify: `docs/superpowers/plans/[12]2026-07-01-user-quiet-hours-digest.md`

- [x] **Step 1: Re-read plan [12] acceptance criteria**
- [x] **Step 2: Map each criterion to code, tests, or docs evidence**
- [x] **Step 3: Fix any scoped gap found by the audit**
- [x] **Step 4: Mark completed plan [12] tasks only after evidence exists**

## Task 2: User Notification Regression Audit

**Files:**
- Inspect: `internal/api/user.go`
- Inspect: `internal/rule/engine.go`
- Inspect: `internal/scheduler/user_digest.go`
- Inspect: `internal/storage/user_repo.go`
- Inspect: `internal/storage/alert_rule_repo.go`

- [x] **Step 1: Confirm session-scoped preference, delivery, unbind, and log APIs**
- [x] **Step 2: Confirm quiet-hours suppression logs `quiet_hours` without sending**
- [x] **Step 3: Confirm digest queue writes `digested` and scheduled flush sends summary**
- [x] **Step 4: Confirm operator/admin notification paths are unchanged**

## Task 3: Documentation And Master Plan Closeout

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`

- [x] **Step 1: Confirm README documents quiet-hours and digest APIs**
- [x] **Step 2: Confirm user manual documents quiet-hours and digest behavior**
- [x] **Step 3: Update the master plan current-next-step from plan [12] to post-closeout**
- [x] **Step 4: Record any verification caveat directly in plan [13] execution notes**

## Task 4: Final Acceptance Suite

**Files:**
- Inspect: full repository

- [x] **Step 1: Run Dashboard JS syntax check**

```bash
node --check internal/api/dashboardui/app.js
```

- [x] **Step 2: Run full Go test suite**

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
```

- [x] **Step 3: Run targeted integration tests**

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test -tags integration ./internal/integration -run "UserNotificationPreferences|UserTelegramUnbind|UserDeliveryPreference|TelegramBindingRepositories|AuthRepositories" -v'
```

- [x] **Step 4: Run Docker Compose smoke**

```bash
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
```

- [x] **Step 5: Verify Dashboard route**

```bash
curl -fsS http://127.0.0.1:18080/dashboard
```

- [x] **Step 6: Run diff whitespace check**

```bash
git diff --check
```

Expected: all commands PASS. If Docker escalation is unavailable, record the exact approval or quota blocker and leave plan [13] unchecked.

## Execution Notes

- 2026-07-01: Plan created after implementation work for `[12]2026-07-01-user-quiet-hours-digest.md`. Scope is closeout, verification, and any narrowly required fixes found during acceptance.
- 2026-07-01: Static closeout found and fixed Redis digest time-window pruning so digest queues are bounded by user, time window, and item count.
- 2026-07-01: Final acceptance passed with `node --check internal/api/dashboardui/app.js`; Docker Go 1.24 `go test ./...`; targeted real PostgreSQL/Redis integration `CW_INTEGRATION_TESTS=1 go test -tags integration ./internal/integration -run "UserNotificationPreferences|UserTelegramUnbind|UserDeliveryPreference|TelegramBindingRepositories|AuthRepositories" -v`; `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh`; `curl -fsS http://127.0.0.1:18080/dashboard`; and `git diff --check`.
