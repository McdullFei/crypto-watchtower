package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMigrationRunnerExecutesAndRecordsNewMigration(t *testing.T) {
	db := newFakeMigrationDB()
	runner := NewMigrationRunner(db, map[string]string{
		"001_init.sql": "CREATE TABLE users (id BIGSERIAL PRIMARY KEY);",
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	if !db.executedSQL("CREATE TABLE users") {
		t.Fatal("expected migration SQL to be executed")
	}
	if !db.executedSQL("INSERT INTO schema_migrations") {
		t.Fatal("expected migration to be recorded")
	}
}

func TestMigrationRunnerSkipsAppliedMigration(t *testing.T) {
	db := newFakeMigrationDB()
	db.applied = append(db.applied, "001_init.sql")
	runner := NewMigrationRunner(db, map[string]string{
		"001_init.sql": "CREATE TABLE users (id BIGSERIAL PRIMARY KEY);",
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	if db.executedSQL("CREATE TABLE users") {
		t.Fatal("expected applied migration to be skipped")
	}
}

func TestMigrationRunnerReturnsExecutionError(t *testing.T) {
	db := newFakeMigrationDB()
	db.execErr = errors.New("boom")
	runner := NewMigrationRunner(db, map[string]string{
		"001_init.sql": "CREATE TABLE users (id BIGSERIAL PRIMARY KEY);",
	})

	if err := runner.Run(context.Background()); err == nil {
		t.Fatal("expected migration execution error")
	}
}

// TestAuthMigrationAddsSessionAndPasswordTables verifies auth schema changes are present.
//
// Author: monsterfei
// Date: 2026-06-30
func TestAuthMigrationAddsSessionAndPasswordTables(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/004_auth_subscription.sql")
	if err != nil {
		t.Fatalf("read auth migration: %v", err)
	}
	sql := string(raw)
	for _, expected := range []string{
		"ADD COLUMN IF NOT EXISTS password_hash",
		"CREATE TABLE IF NOT EXISTS user_sessions",
		"CREATE TABLE IF NOT EXISTS password_reset_tokens",
		"idx_users_email_lower",
		"idx_user_sessions_token_hash",
		"idx_password_reset_tokens_token_hash",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected %q in auth migration", expected)
		}
	}
}

// TestTelegramBindingMigrationAddsTokenTable verifies Telegram binding schema changes are present.
//
// Author: monsterfei
// Date: 2026-07-01
func TestTelegramBindingMigrationAddsTokenTable(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/005_user_telegram_binding.sql")
	if err != nil {
		t.Fatalf("read telegram binding migration: %v", err)
	}
	sql := string(raw)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS telegram_binding_tokens",
		"token_hash CHAR(64) NOT NULL",
		"idx_telegram_binding_tokens_token_hash",
		"idx_telegram_binding_tokens_user_expires",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected %q in telegram binding migration", expected)
		}
	}
}

// TestUserDeliveryPreferenceMigrationAddsTelegramSwitch verifies user delivery preference schema changes are present.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserDeliveryPreferenceMigrationAddsTelegramSwitch(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/006_user_delivery_preferences.sql")
	if err != nil {
		t.Fatalf("read user delivery preference migration: %v", err)
	}
	sql := string(raw)
	for _, expected := range []string{
		"ADD COLUMN IF NOT EXISTS telegram_delivery_enabled",
		"BOOLEAN NOT NULL DEFAULT TRUE",
		"idx_users_telegram_delivery_enabled",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected %q in user delivery preference migration", expected)
		}
	}
}

// TestUserNotificationPreferencesMigrationAddsQuietHoursAndDigest verifies quiet-hours and digest schema changes are present.
//
// Author: monsterfei
// Date: 2026-07-01
func TestUserNotificationPreferencesMigrationAddsQuietHoursAndDigest(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/007_user_notification_preferences.sql")
	if err != nil {
		t.Fatalf("read user notification preference migration: %v", err)
	}
	sql := string(raw)
	for _, expected := range []string{
		"ADD COLUMN IF NOT EXISTS telegram_quiet_hours_enabled",
		"ADD COLUMN IF NOT EXISTS telegram_quiet_hours_start",
		"ADD COLUMN IF NOT EXISTS telegram_quiet_hours_end",
		"ADD COLUMN IF NOT EXISTS telegram_quiet_hours_timezone",
		"ADD COLUMN IF NOT EXISTS telegram_digest_enabled",
		"ADD COLUMN IF NOT EXISTS telegram_digest_interval_min",
		"idx_users_telegram_digest_enabled",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected %q in user notification preference migration", expected)
		}
	}
}

type fakeMigrationDB struct {
	applied []string
	execs   []string
	execErr error
}

func newFakeMigrationDB() *fakeMigrationDB {
	return &fakeMigrationDB{applied: []string{}}
}

func (f *fakeMigrationDB) Exec(_ context.Context, sql string, args ...any) error {
	f.execs = append(f.execs, sql)
	if f.execErr != nil && !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS schema_migrations") {
		return f.execErr
	}
	if strings.Contains(sql, "INSERT INTO schema_migrations") && len(args) > 0 {
		f.applied = append(f.applied, args[0].(string))
	}
	return nil
}

func (f *fakeMigrationDB) AppliedMigrations(context.Context) (map[string]bool, error) {
	out := make(map[string]bool, len(f.applied))
	for _, name := range f.applied {
		out[name] = true
	}
	return out, nil
}

func (f *fakeMigrationDB) executedSQL(fragment string) bool {
	for _, sql := range f.execs {
		if strings.Contains(sql, fragment) {
			return true
		}
	}
	return false
}
