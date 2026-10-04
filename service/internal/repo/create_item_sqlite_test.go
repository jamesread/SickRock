package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestCreateItemInTableSQLiteUsesDatetimeNow(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE table_configurations (
			name TEXT PRIMARY KEY,
			db TEXT,
			"table" TEXT,
			title TEXT,
			ordinal INTEGER DEFAULT 0,
			create_button_text TEXT,
			row_name TEXT,
			create_delegate TEXT,
			icon TEXT,
			primary_key_column TEXT,
			default_sort_column TEXT
		);
		INSERT INTO table_configurations (name, db, "table", title) VALUES ('items', 'main', 'items', 'Items');
		CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			sr_created TEXT,
			sr_updated TEXT
		);
	`); err != nil {
		t.Fatalf("setup schema: %v", err)
	}

	r := NewRepository(db)
	item, err := r.CreateItemInTable(context.Background(), "items", map[string]string{"name": "alpha"})
	if err != nil {
		t.Fatalf("CreateItemInTable: %v", err)
	}
	if item.ID == "" {
		t.Fatal("expected item id")
	}
	if item.Fields["name"] != "alpha" {
		t.Fatalf("unexpected name field: %v", item.Fields["name"])
	}

	var created string
	if err := db.Get(&created, "SELECT sr_created FROM items WHERE id = ?", item.ID); err != nil {
		t.Fatalf("read sr_created: %v", err)
	}
	if created == "" {
		t.Fatal("expected sr_created to be set")
	}
}
