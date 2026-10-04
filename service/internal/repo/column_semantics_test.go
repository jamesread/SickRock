package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestApplyColumnSemanticsUserRef(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE table_column_semantics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_configuration TEXT NOT NULL,
			column_name TEXT NOT NULL,
			semantic_type TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE (table_configuration, column_name)
		);
		INSERT INTO table_column_semantics (table_configuration, column_name, semantic_type)
		VALUES ('tasks', 'owner_id', 'user_ref');
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	r := NewRepository(db)
	specs := []FieldSpec{{Name: "owner_id", Type: "BIGINT", Required: false}}
	out := r.ApplyColumnSemantics(context.Background(), "tasks", specs)
	if out[0].Type != SemanticTypeUserRef {
		t.Fatalf("expected user_ref type, got %q", out[0].Type)
	}
}

func TestSQLiteDatetimeColumnDeclaredType(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE events (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	r := NewRepository(db)
	if err := r.AddColumn(context.Background(), "main", "events", FieldSpec{
		Name: "event_start",
		Type: "datetime",
	}); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}

	var declared string
	if err := db.Get(&declared, `SELECT type FROM pragma_table_info('events') WHERE name = 'event_start'`); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if declared != "DATETIME" {
		t.Fatalf("declared type %q, want DATETIME", declared)
	}

	if _, err := db.Exec(`INSERT INTO events (event_start) VALUES ('2026-01-01 10:00:00')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var storage string
	if err := db.Get(&storage, `SELECT typeof(event_start) FROM events`); err != nil {
		t.Fatalf("typeof: %v", err)
	}
	if storage != "text" {
		t.Fatalf("storage class %q, want text", storage)
	}
}
