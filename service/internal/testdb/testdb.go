// Package testdb provides databases for tests that need the real SickRock schema.
package testdb

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// NewMigratedSQLite returns a fresh SQLite database with the real migrations applied, so
// test schemas cannot drift from production. A file (not :memory:) is used because
// repository code can hold two connections at once.
func NewMigratedSQLite(tb testing.TB) *sqlx.DB {
	tb.Helper()
	db, err := sqlx.Open("sqlite", "file:"+filepath.Join(tb.TempDir(), "test.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		tb.Fatalf("open: %v", err)
	}
	tb.Cleanup(func() { _ = db.Close() })
	applySQLiteMigrations(tb, db)
	return db
}

func applySQLiteMigrations(tb testing.TB, db *sqlx.DB) {
	tb.Helper()
	driver, err := sqlite3.WithInstance(db.DB, &sqlite3.Config{})
	if err != nil {
		tb.Fatalf("migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir(), "sqlite3", driver)
	if err != nil {
		tb.Fatalf("migrate: %v", err)
	}
	if err := m.Up(); err != nil {
		tb.Fatalf("migrate up: %v", err)
	}
}

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations", "sqlite")
}
