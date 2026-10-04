package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestLookupTableConfigNames(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := configureSQLiteDB(db); err != nil {
		t.Fatalf("configure: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE table_configurations (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			"db" TEXT,
			"table" TEXT,
			title TEXT,
			ordinal INTEGER
		);
		INSERT INTO table_configurations (name, "db", "table", title, ordinal) VALUES
			('customers', 'main', 'customers', 'Customers', 0),
			('orders', 'main', 'orders', 'Orders', 1);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	r := NewRepository(db)
	names, err := r.LookupTableConfigNames(context.Background(), []DbTablePair{
		{Db: "main", Table: "customers"},
		{Db: "main", Table: "orders"},
		{Db: "main", Table: "missing"},
	})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if names[DbTableLookupKey("main", "customers")] != "customers" {
		t.Fatalf("customers: %#v", names)
	}
	if names[DbTableLookupKey("main", "orders")] != "orders" {
		t.Fatalf("orders: %#v", names)
	}
}
