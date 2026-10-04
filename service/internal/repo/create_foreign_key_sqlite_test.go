package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestCreateForeignKeySQLite(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := configureSQLiteDB(db); err != nil {
		t.Fatalf("configure: %v", err)
	}
	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE customers (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("customers: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE opportunities (id INTEGER PRIMARY KEY, customer INTEGER)`)
	if err != nil {
		t.Fatalf("opportunities: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE table_configurations (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			"db" TEXT,
			"table" TEXT,
			title TEXT,
			ordinal INTEGER,
			create_button_text TEXT,
			create_delegate TEXT,
			icon TEXT,
			primary_key_column TEXT,
			default_sort_column TEXT,
			row_name TEXT
		)`)
	if err != nil {
		t.Fatalf("table_configurations: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO table_configurations (name, "db", "table", title, ordinal) VALUES
			('customers', 'main', 'customers', 'Customers', 0),
			('opportunities', 'main', 'opportunities', 'Opportunities', 1)`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	_, err = db.Exec(`
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
		)`)
	if err != nil {
		t.Fatalf("table_relations: %v", err)
	}

	r := NewRepository(db)
	if err := r.CreateForeignKey(context.Background(),
		"opportunities", "customer", "customers", "id", "RESTRICT", "RESTRICT"); err != nil {
		t.Fatalf("CreateForeignKey: %v", err)
	}

	var relCount int
	if err := db.Get(&relCount, "SELECT COUNT(*) FROM table_relations WHERE source_table_key = 'opportunities'"); err != nil {
		t.Fatalf("relations: %v", err)
	}
	if relCount < 1 {
		t.Fatalf("expected table_relations row, got %d", relCount)
	}
	var fkCount int
	if err := db.Get(&fkCount, "SELECT COUNT(*) FROM pragma_foreign_key_list('opportunities')"); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if fkCount < 1 {
		t.Fatalf("expected foreign key on opportunities, got %d", fkCount)
	}
}

func TestRebuildSQLiteTableAddForeignKey_DatetimeDefaults(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := configureSQLiteDB(db); err != nil {
		t.Fatalf("configure: %v", err)
	}
	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE customers (id INTEGER PRIMARY KEY AUTOINCREMENT)`)
	if err != nil {
		t.Fatalf("customers: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE opportunities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sr_created TEXT DEFAULT (datetime('now')),
		sr_updated TEXT DEFAULT (datetime('now')),
		customer BIGINT
	)`)
	if err != nil {
		t.Fatalf("opportunities: %v", err)
	}

	r := NewRepository(db)
	if err := r.rebuildSQLiteTableAddForeignKey(context.Background(),
		"opportunities", "customer", "customers", "id", "RESTRICT", "RESTRICT"); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	var fkCount int
	if err := db.Get(&fkCount, "SELECT COUNT(*) FROM pragma_foreign_key_list('opportunities')"); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if fkCount < 1 {
		t.Fatalf("expected foreign key after rebuild, got %d", fkCount)
	}
}

func TestSQLiteDefaultClause(t *testing.T) {
	if sqliteDefaultClause("datetime('now')") != "DEFAULT (datetime('now'))" {
		t.Fatalf("expression default: %q", sqliteDefaultClause("datetime('now')"))
	}
	if sqliteDefaultClause("'x'") != "DEFAULT 'x'" {
		t.Fatalf("literal default: %q", sqliteDefaultClause("'x'"))
	}
}
