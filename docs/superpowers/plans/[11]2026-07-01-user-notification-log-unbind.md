# User Notification Log And Unbind Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give users a bounded view of their own notification delivery logs and a safe way to unbind Telegram without affecting operator/admin notifications.

**Architecture:** Keep existing user alert history and admin notification APIs unchanged. Add session-scoped user notification-log reads, add a Telegram unbind endpoint that clears only the current user's binding, and surface both in the Dashboard. All list queries must be bounded and must derive user identity from the session.

**Tech Stack:** Go 1.24, standard `net/http`, PostgreSQL 16.14 repositories, embedded Dashboard UI, Docker Compose smoke.

---

## Acceptance Criteria

- [ ] Logged-in users can list their own recent notification logs with bounded `limit`.
- [ ] User notification-log responses include channel, masked target, status, error_message, alert_id, and created_at.
- [ ] Users cannot request another user's notification logs by query/body user id.
- [ ] Logged-in users can unbind Telegram from their own account.
- [ ] Unbind clears `telegram_chat_id` but preserves `telegram_delivery_enabled` preference.
- [ ] Dashboard shows recent notification-log rows and a Telegram unbind control.
- [ ] Operator Bearer Token notification-log APIs keep current behavior.
- [ ] Verification gate passes.

## Scope Boundary

- This plan includes user notification-log list API, Dashboard notification-log panel, Telegram self-service unbind, and docs.
- This plan does not include notification retries, digest scheduling, quiet hours, email/SMS/mobile push, teams, or billing changes.

## File Structure

- Modify: `internal/storage/notification_log_repo.go` - bounded user log listing with target masking handled above the repository.
- Modify: `internal/storage/user_repo.go` - clear Telegram binding for one user.
- Modify: `internal/user/service.go` - user notification-log reads and Telegram unbind coordination.
- Modify: `internal/api/user.go` - session-protected log list and unbind endpoints.
- Modify: `internal/api/user_test.go` - API authorization and scoping tests.
- Modify: `internal/api/dashboardui/*` - notification-log panel and unbind control.
- Modify: `README.md`, `docs/用户手册.md`, `docs/plan/币圈异动监控平台总体开发计划.md` - document behavior.

## Task 1: User Notification Log Reads

**Files:**
- Modify: `internal/storage/notification_log_repo.go`
- Modify: `internal/user/service.go`
- Modify: `internal/user/service_test.go`

- [ ] **Step 1: Write failing bounded user notification-log service tests**
- [ ] **Step 2: Implement repository and service read path**
- [ ] **Step 3: Mask notification targets before API exposure**
- [ ] **Step 4: Run user and storage tests**

## Task 2: User Notification Log API

**Files:**
- Modify: `internal/api/user.go`
- Modify: `internal/api/user_test.go`

- [ ] **Step 1: Write failing session-scoped notification-log API tests**
- [ ] **Step 2: Implement `/api/v1/user/notifications` GET**
- [ ] **Step 3: Ensure query/body user ids are ignored**
- [ ] **Step 4: Run API tests**

## Task 3: Telegram Unbind

**Files:**
- Modify: `internal/storage/user_repo.go`
- Modify: `internal/user/service.go`
- Modify: `internal/api/user.go`
- Modify: `internal/api/user_test.go`
- Modify: `internal/integration/postgres_redis_test.go`

- [ ] **Step 1: Write failing Telegram unbind repository and API tests**
- [ ] **Step 2: Implement current-user Telegram unbind**
- [ ] **Step 3: Preserve Telegram delivery preference when unbinding**
- [ ] **Step 4: Run API and integration tests**

## Task 4: Dashboard UX

**Files:**
- Modify: `internal/api/dashboardui/index.html`
- Modify: `internal/api/dashboardui/app.js`
- Modify: `internal/api/dashboardui/styles.css`
- Modify: `internal/api/user_test.go`

- [ ] **Step 1: Write failing Dashboard markup test**
- [ ] **Step 2: Render recent notification logs**
- [ ] **Step 3: Add Telegram unbind control and refresh state**
- [ ] **Step 4: Run API tests and `node --check`**

## Task 5: Docs And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`
- Modify: `docs/superpowers/plans/[11]2026-07-01-user-notification-log-unbind.md`

- [ ] **Step 1: Document user notification logs and unbind behavior**
- [ ] **Step 2: Update master plan status**
- [ ] **Step 3: Run verification gate**

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

- 2026-07-01: Plan created after `[10]2026-07-01-user-rule-window-delivery-controls.md` implementation. Scope is user-facing notification-log visibility and Telegram unbind only; retries, quiet hours, digest scheduling, and new notification channels remain out of scope.
