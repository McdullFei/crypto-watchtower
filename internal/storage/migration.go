package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MigrationDB interface {
	Exec(ctx context.Context, sql string, args ...any) error
	AppliedMigrations(ctx context.Context) (map[string]bool, error)
}

type MigrationRunner struct {
	db         MigrationDB
	migrations map[string]string
}

func NewMigrationRunner(db MigrationDB, migrations map[string]string) MigrationRunner {
	copied := make(map[string]string, len(migrations))
	for name, sql := range migrations {
		copied[name] = sql
	}
	return MigrationRunner{db: db, migrations: copied}
}

func NewFileMigrationRunner(db MigrationDB, dir string) (MigrationRunner, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return MigrationRunner{}, err
	}

	migrations := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return MigrationRunner{}, err
		}
		migrations[entry.Name()] = string(raw)
	}
	return NewMigrationRunner(db, migrations), nil
}

func (r MigrationRunner) Run(ctx context.Context) error {
	if err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, err := r.db.AppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}

	names := make([]string, 0, len(r.migrations))
	for name := range r.migrations {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if applied[name] {
			continue
		}
		if err := r.db.Exec(ctx, r.migrations[name]); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err := r.db.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}

type PostgresMigrationDB struct {
	Pool *pgxpool.Pool
}

func NewPostgresMigrationDB(pool *pgxpool.Pool) PostgresMigrationDB {
	return PostgresMigrationDB{Pool: pool}
}

func (db PostgresMigrationDB) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := db.Pool.Exec(ctx, sql, args...)
	return err
}

func (db PostgresMigrationDB) AppliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := db.Pool.Query(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, err
		}
		out[filename] = true
	}
	return out, rows.Err()
}
