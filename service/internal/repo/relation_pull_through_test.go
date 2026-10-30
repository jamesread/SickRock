package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestUpdateTableRelationPullThrough(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	_, err = db.ExecContext(ctx, `
		CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT);
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
			row_name_plural TEXT,
			icon TEXT,
			primary_key_column TEXT,
			default_sort_column TEXT,
			row_actions TEXT,
			bulk_entry_json TEXT
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
			display_name TEXT,
			pull_through_json TEXT,
			absorb_into_item INTEGER NOT NULL DEFAULT 0,
			absorb_type TEXT
		);
		INSERT INTO table_relations (
			constraint_name, source_table_key, source_column,
			referenced_table_key, referenced_column
		) VALUES ('fk_orders_customer_id_customers_id', 'orders', 'customer_id', 'customers', 'id');
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	repo := NewRepository(db)
	if err = repo.UpdateTableRelation(ctx, "fk_orders_customer_id_customers_id", "Customer orders", []RelationPullThroughField{
		{FKColumn: "customer_id", Field: "name"},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	rel, err := repo.GetTableRelationByConstraintName(ctx, "fk_orders_customer_id_customers_id")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !rel.PullThroughJSON.Valid || rel.PullThroughJSON.String == "" {
		t.Fatalf("expected pull_through_json, got %+v", rel.PullThroughJSON)
	}
	fks, err := repo.GetForeignKeys(ctx, "orders")
	if err != nil {
		t.Fatalf("GetForeignKeys: %v", err)
	}
	if len(fks) != 1 || len(fks[0].PullThroughFields) != 1 || fks[0].PullThroughFields[0].Field != "name" {
		t.Fatalf("unexpected fk pull-through: %+v", fks[0].PullThroughFields)
	}
}
