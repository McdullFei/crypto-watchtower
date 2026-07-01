# User Custom Rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backend support for user-scoped alert rules without changing the existing system-rule real-time alert behavior.

**Architecture:** Keep the current `rule.Engine` driven by system rules only, because the runtime alert pipeline does not yet have user-specific notification fanout. Add user-rule persistence, filtering, and protected API write/list paths so the later user dashboard can manage user rules safely. Store user rules in the existing `alert_rules` table with `scope='user'` and `user_id` set.

**Tech Stack:** Go 1.24, PostgreSQL, existing `storage.AlertRuleRepo`, existing `api` router, Docker Compose smoke.

---

## Acceptance Criteria

- [x] Protected `POST /api/v1/rules` accepts `user_id` and stores a user-scoped rule.
- [x] `GET /api/v1/rules?user_id=<id>` returns user rules without replacing the default system-rule response.
- [x] Admin rule listing can filter by `scope` and `user_id`.
- [x] User rules are upserted by `(user_id, scope, exchange, symbol, rule_type)` and do not create duplicates.
- [x] Runtime system rule loading remains system-only, so user rules do not accidentally alter global alert thresholds.
- [x] Verification gate passes.

## File Structure

- Modify: `internal/storage/list_filter.go` - add `Scope` and `UserID` filters.
- Modify: `internal/storage/alert_rule_repo.go` - add user rule upsert/list methods and list filters.
- Create: `migrations/003_user_rule_unique.sql` - add unique index for user rules.
- Modify: `internal/rule/service.go` and `internal/rule/service_test.go` - expose user-rule operations and prove they do not mutate global engine state.
- Modify: `internal/api/rules.go` and `internal/api/health_test.go` - support `user_id` in protected rule writes and filtered reads.
- Modify: `internal/api/admin.go`, `internal/api/admin_test.go`, and `internal/admin/service.go` - support Admin rule filters.
- Modify: `README.md`, `docs/用户手册.md`, and `docs/plan/币圈异动监控平台总体开发计划.md` - document user custom rule backend status.

## Task 1: API Contract Tests

**Files:**
- Modify: `internal/api/health_test.go`
- Modify: `internal/api/rules.go`

- [x] **Step 1: Write failing API tests**

Add tests proving:

```go
func TestRulesPostUpsertsUserRule(t *testing.T)
func TestRulesGetFiltersUserRules(t *testing.T)
```

`TestRulesPostUpsertsUserRule` posts:

```json
{"user_id":42,"exchange":"binance","symbol":"BTCUSDT","rule_type":"large_trade","threshold":120000}
```

Expected: protected request returns `200`, and the stub service receives a rule with `Scope: "user"` and `UserID: 42`.

`TestRulesGetFiltersUserRules` calls:

```text
GET /api/v1/rules?user_id=42
```

Expected: response contains `database_rules`, and the stub service records `lastUserID=42`.

- [x] **Step 2: Run failing API tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run "Rules(PostUpsertsUserRule|GetFiltersUserRules)" -v'
```

Expected: FAIL because `RuleService` does not expose user-rule methods and `ruleWriteRequest` has no `user_id`.

- [x] **Step 3: Implement API support**

Change `RuleService` to include:

```go
ListUserRules(context.Context, int64) ([]model.AlertRule, error)
UpsertUserRule(context.Context, model.AlertRule) error
```

Add `UserID *int64 json:"user_id"` to `ruleWriteRequest`. In `toModel`, when `UserID != nil`, require `*UserID > 0`, set `Scope: "user"`, and set `UserID`. Otherwise keep `Scope: "system"`.

In `handleGetRules`, if query `user_id` is positive, call `ListUserRules`; otherwise call `ListEnabled`.

In `handlePostRules`, call `UpsertUserRule` for user-scoped rules and `UpsertSystemRule` for system rules.

- [x] **Step 4: Run API tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run "Rules(PostUpsertsUserRule|GetFiltersUserRules)" -v'
```

Expected: PASS.

## Task 2: Runtime Service Tests

**Files:**
- Modify: `internal/rule/service.go`
- Modify: `internal/rule/service_test.go`

- [x] **Step 1: Write failing runtime service test**

Add:

```go
func TestRuntimeRuleServiceUpsertUserRuleDoesNotUpdateSystemEngine(t *testing.T)
```

Use a default engine that would emit a `large_trade` alert at `Notional: 150000`. Upsert a user rule for `BTCUSDT/large_trade` with threshold `300000`, then evaluate the same event. Expected: alert is still emitted because user rules must not mutate the global system engine.

- [x] **Step 2: Run failing runtime test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/rule -run UserRule -v'
```

Expected: FAIL because `UpsertUserRule` does not exist.

- [x] **Step 3: Implement runtime service method**

Add `UpsertUserRule` and `ListUserRules` to the repository interface and service. `UpsertUserRule` must persist through the repo but must not call `engine.ApplyRule`.

- [x] **Step 4: Run runtime test**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/rule -run UserRule -v'
```

Expected: PASS.

## Task 3: Storage And Admin Filters

**Files:**
- Modify: `internal/storage/list_filter.go`
- Modify: `internal/storage/alert_rule_repo.go`
- Create: `migrations/003_user_rule_unique.sql`
- Modify: `internal/api/admin.go`
- Modify: `internal/api/admin_test.go`
- Modify: `internal/admin/service.go`
- Modify: `internal/integration/postgres_redis_test.go`

- [x] **Step 1: Write failing Admin filter test**

Add:

```go
func TestAdminRulesParsesUserRuleFilters(t *testing.T)
```

Call:

```text
GET /api/v1/admin/rules?scope=user&user_id=42&limit=10
```

Expected: the stub admin service receives `Scope: "user"` and `UserID: 42`.

- [x] **Step 2: Extend integration coverage**

Add user rule insert/list assertions to `internal/integration/postgres_redis_test.go` after system rule assertions. Insert `UserID: &userID`, `Scope: "user"`, then list with `storage.ListFilter{Scope:"user", UserID:&userID, Limit:1}`.

- [x] **Step 3: Run failing tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run AdminRulesParsesUserRuleFilters -v'
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/integration -tags integration -run PostgresRedis -v'
```

Expected: Admin test FAILS because filter fields are missing. Integration command may skip real dependencies unless `CW_INTEGRATION_TESTS=1`, but it must compile.

- [x] **Step 4: Implement storage and Admin filters**

Add `Scope string` and `UserID *int64` to `storage.ListFilter` and `api.AdminListFilter`. Parse `user_id` in `adminFilterFromRequest`.

In `AlertRuleRepo.List`, add optional filters for `scope` and `user_id`.

Add:

```go
func (r AlertRuleRepo) ListUserRules(ctx context.Context, userID int64) ([]model.AlertRule, error)
func (r AlertRuleRepo) UpsertUserRule(ctx context.Context, rule model.AlertRule) error
```

Create `migrations/003_user_rule_unique.sql` with:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_rules_user_unique
    ON alert_rules(user_id, scope, exchange, symbol, rule_type)
    WHERE user_id IS NOT NULL;
```

- [x] **Step 5: Run storage/Admin tests**

Run:

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/api -run AdminRulesParsesUserRuleFilters -v'
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./internal/integration -tags integration -run PostgresRedis -v'
```

Expected: PASS, with integration possibly skipped unless explicitly enabled.

## Task 4: Docs And Status

**Files:**
- Modify: `README.md`
- Modify: `docs/用户手册.md`
- Modify: `docs/plan/币圈异动监控平台总体开发计划.md`

- [x] **Step 1: Document user-rule API**

Document that protected `POST /api/v1/rules` accepts optional `user_id`, and `GET /api/v1/rules?user_id=<id>` lists user rules.

- [x] **Step 2: Update master plan**

Mark Phase 3 user custom rules complete only after the verification gate passes.

## Verification Gate

- [x] `docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24 sh -c '/usr/local/go/bin/go test ./...'`
- [x] `APP_HTTP_PORT=18080 ./scripts/smoke-docker-compose.sh`
- [x] `git diff --check`

## Execution Notes

- 2026-06-30: Plan created after completing `[5]2026-06-29-ai-market-summary.md`; implementation starts in the same session.
- 2026-06-30: API and runtime service tests passed before Docker execution became unavailable. Later Docker/gofmt/test commands were blocked by the Codex usage limit, and local `gofmt` is not installed, so final verification remains pending.
- 2026-06-30: `git diff --check` passed after the user-rule code and docs changes.
- 2026-06-30: Docker became available again; ran gofmt, API user-rule tests, runtime user-rule test, Admin user-rule filter test, full `go test ./...`, Docker Compose smoke on port 18080, and `git diff --check`.
- 2026-06-30: Re-ran PostgreSQL/Redis integration with `CW_INTEGRATION_TESTS=1` inside the compose network. The first attempt exposed a DSN password mismatch, the second attempt exposed that the app image lacks `printenv`, and the final `docker inspect`-based run passed against real PostgreSQL and Redis without skipped tests.
