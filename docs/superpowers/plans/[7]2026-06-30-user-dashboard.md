# User Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a lightweight user-facing dashboard for viewing and managing personal rules, alert history, and notification binding state.

**Architecture:** Keep the existing `/admin` operator console separate from the user dashboard. Add user-facing read/write APIs under `/api/v1/user/*` backed by existing `users`, `alert_rules`, `alerts`, and `notification_logs` tables, then add a simple embedded `/dashboard` page. Authentication can initially use the existing Bearer Token operator guard plus explicit `user_id` until SaaS login is implemented in a later plan.

**Tech Stack:** Go 1.24, standard `net/http`, embedded HTML/CSS/JavaScript, PostgreSQL repositories, existing Docker Compose smoke.

---

## Acceptance Criteria

- [x] `/dashboard` serves a user-facing page separate from `/admin`.
- [x] User dashboard can list personal rules through `GET /api/v1/user/rules?user_id=<id>`.
- [x] User dashboard can create/update personal rules through `POST /api/v1/user/rules`.
- [x] User dashboard can list personal alert history through `GET /api/v1/user/alerts?user_id=<id>`.
- [x] User dashboard can show Telegram binding state from `users.telegram_chat_id`.
- [x] User APIs remain protected by Bearer Token until SaaS login exists.
- [x] User dashboard does not expose operator-only Admin APIs.
- [x] Verification gate passes.

## Dependency

- [x] Complete and verify `[6]2026-06-30-user-custom-rules.md` before starting this plan.

## File Structure

- Create: `internal/api/user.go` - user-facing API routes and DTOs.
- Create: `internal/api/user_test.go` - API behavior tests.
- Modify: `internal/api/router.go` - mount user routes and `/dashboard` assets.
- Create: `internal/api/dashboardui/index.html` - embedded user dashboard shell.
- Create: `internal/api/dashboardui/styles.css` - dashboard styling.
- Create: `internal/api/dashboardui/app.js` - dashboard API client and interactions.
- Modify: `internal/storage/user_repo.go` - add lookup helpers for dashboard binding state if missing.
- Modify: `internal/storage/alert_repo.go` - add user alert listing only after the alert ownership model is defined.
- Modify: `README.md` and `docs/用户手册.md` - document dashboard usage.

## Task 1: User API Contract

**Files:**
- Create: `internal/api/user_test.go`
- Create: `internal/api/user.go`
- Modify: `internal/api/router.go`

- [x] **Step 1: Write failing user API tests**

Add tests:

```go
func TestUserRulesRequireBearerToken(t *testing.T)
func TestUserRulesListReturnsUserRules(t *testing.T)
func TestUserRulesPostWritesUserRule(t *testing.T)
```

Expected behavior:

- Missing Bearer Token returns `401`.
- `GET /api/v1/user/rules?user_id=42` returns only rules for user `42`.
- `POST /api/v1/user/rules` accepts `user_id`, `exchange`, `symbol`, `rule_type`, `threshold`, `window_sec`, and `enabled`.

- [x] **Step 2: Run failing tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run UserRules -v'
```

Expected: FAIL because user routes do not exist.

- [x] **Step 3: Implement user rule routes**

Add user API handlers that reuse the `RuleService.ListUserRules` and `RuleService.UpsertUserRule` methods from plan `[6]`.

- [x] **Step 4: Run user API tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run UserRules -v'
```

Expected: PASS.

## Task 2: Binding State API

**Files:**
- Modify: `internal/storage/user_repo.go`
- Create or modify: `internal/api/user_test.go`
- Modify: `internal/api/user.go`

- [x] **Step 1: Write failing binding state test**

Add:

```go
func TestUserProfileReturnsTelegramBindingState(t *testing.T)
```

Expected: `GET /api/v1/user/profile?user_id=42` returns `telegram_bound=true` when `users.telegram_chat_id` is present.

- [x] **Step 2: Implement repository and API helper**

Add a bounded user lookup by ID, then expose `telegram_bound` and masked binding metadata.

- [x] **Step 3: Run binding tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api ./internal/storage -run UserProfile -v'
```

Expected: PASS.

## Task 3: Dashboard UI

**Files:**
- Create: `internal/api/dashboardui/index.html`
- Create: `internal/api/dashboardui/styles.css`
- Create: `internal/api/dashboardui/app.js`
- Modify: `internal/api/router.go`

- [x] **Step 1: Add dashboard shell**

Create a practical user dashboard with:

- Bearer Token input.
- User ID input.
- Personal rules table.
- Rule editor form.
- Telegram binding status panel.
- Alert history placeholder panel if user-alert ownership is not yet available.

- [x] **Step 2: Add embedded route**

Serve `/dashboard` and `/dashboard/*` from embedded assets.

- [x] **Step 3: Run UI route tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run Dashboard -v'
```

Expected: PASS.

## Task 4: Docs And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`

- [x] **Step 1: Document dashboard usage**

Document `/dashboard`, Bearer Token usage, user id input, and the limitation that SaaS login is still a later plan.

- [x] **Step 2: Run verification gate**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'
APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh
git diff --check
```

Expected: PASS.

## Execution Notes

- 2026-06-30: Plan created after starting `[6]2026-06-30-user-custom-rules.md`.
- 2026-06-30: `[6]` passed verification; this plan is ready to execute next.
- 2026-06-30: User Dashboard implementation completed with full Go tests, Docker Compose smoke, `git diff --check`, and live `/dashboard` HTTP check.
