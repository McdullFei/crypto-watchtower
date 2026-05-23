package storage

import (
	"context"
	"errors"
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
