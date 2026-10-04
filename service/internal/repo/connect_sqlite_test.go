package repo

import (
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestConfigureSQLiteDB(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := configureSQLiteDB(db); err != nil {
		t.Fatalf("configureSQLiteDB: %v", err)
	}

	var mode string
	if err := db.Get(&mode, "PRAGMA journal_mode"); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != "wal" && mode != "memory" {
		t.Fatalf("expected wal or memory journal_mode, got %q", mode)
	}

	if db.Stats().MaxOpenConnections != sqliteMaxOpenConns {
		t.Fatalf("expected max open %d, got %d", sqliteMaxOpenConns, db.Stats().MaxOpenConnections)
	}
}

func TestConfigureSQLiteDBConcurrentReads(t *testing.T) {
	// Shared-cache memory DB so pooled connections see the same schema.
	db, err := sqlx.Open("sqlite", "file:concurrenttest?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := configureSQLiteDB(db); err != nil {
		t.Fatalf("configureSQLiteDB: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, n TEXT); DELETE FROM t; INSERT INTO t (n) VALUES ('ok');`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < sqliteMaxOpenConns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var n string
			if err := db.Get(&n, "SELECT n FROM t LIMIT 1"); err != nil {
				t.Errorf("read: %v", err)
			}
		}()
	}
	wg.Wait()
}
