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
func NewMigratedSQLite(t testing.TB) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "test.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	driver, err := sqlite3.WithInstance(db.DB, &sqlite3.Config{})
	if err != nil {
		t.Fatalf("migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir(), "sqlite3", driver)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return db
}

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations", "sqlite")
}
