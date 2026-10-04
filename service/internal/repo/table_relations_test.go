package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestGetForeignKeysFromTableRelations(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := configureSQLiteDB(db); err != nil {
		t.Fatalf("configure: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE customers (id INTEGER PRIMARY KEY);
		CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER);
		CREATE TABLE table_configurations (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			"db" TEXT,
			"table" TEXT,
			title TEXT,
			ordinal INTEGER,
			create_button_text TEXT,
			create_delegate TEXT,
			row_name TEXT,
			icon TEXT,
			primary_key_column TEXT,
			default_sort_column TEXT
		);
		INSERT INTO table_configurations (name, "db", "table", title, ordinal) VALUES
			('customers', 'main', 'customers', 'Customers', 0),
			('orders', 'main', 'orders', 'Orders', 1);
		CREATE TABLE table_relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			constraint_name TEXT NOT NULL UNIQUE,
			source_table_key TEXT NOT NULL,
			source_column TEXT NOT NULL,
			referenced_table_key TEXT NOT NULL,
			referenced_column TEXT NOT NULL,
			on_delete_action TEXT NOT NULL DEFAULT 'NO ACTION',
			on_update_action TEXT NOT NULL DEFAULT 'NO ACTION',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO table_relations (
			constraint_name, source_table_key, source_column,
			referenced_table_key, referenced_column, on_delete_action, on_update_action
		) VALUES (
			'fk_orders_customer_id_customers_id',
			'orders', 'customer_id', 'customers', 'id', 'RESTRICT', 'RESTRICT'
		);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	r := NewRepository(db)
	fks, err := r.GetForeignKeys(context.Background(), "customers")
	if err != nil {
		t.Fatalf("GetForeignKeys: %v", err)
	}
	if len(fks) != 1 {
		t.Fatalf("expected 1 relation involving customers, got %d", len(fks))
	}
	if fks[0].ReferencedTable != "customers" || fks[0].TableName != "orders" {
		t.Fatalf("unexpected fk mapping: %+v", fks[0])
	}
}
