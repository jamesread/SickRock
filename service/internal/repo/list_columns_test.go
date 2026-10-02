package repo

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestListColumnsSQLiteScansPrimaryKeyMetadata(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE configured_items (
			item_key TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TEXT
		)
	`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	r := NewRepository(db)
	columns, err := r.ListColumns(context.Background(), &TableConfig{
		Db:    sql.NullString{String: "main", Valid: true},
		Table: sql.NullString{String: "configured_items", Valid: true},
	})
	if err != nil {
		t.Fatalf("list columns: %v", err)
	}

	if len(columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(columns))
	}
	if columns[0].Name != "item_key" || columns[1].Name != "name" {
		t.Fatalf("unexpected columns: %#v", columns)
	}
	if !columns[1].Required {
		t.Fatal("expected name to be required")
	}
}
