package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestGetDatabaseTablesSQLite(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE table_configurations (
			name TEXT PRIMARY KEY,
			db TEXT,
			"table" TEXT
		);
		CREATE TABLE my_data (id INTEGER PRIMARY KEY);
		CREATE TABLE other_data (id INTEGER PRIMARY KEY);
		INSERT INTO table_configurations (name, db, "table") VALUES ('my_data_cfg', 'main', 'my_data');
	`); err != nil {
		t.Fatalf("setup schema: %v", err)
	}

	r := NewRepository(db)
	tables, err := r.GetDatabaseTables(context.Background(), "main")
	if err != nil {
		t.Fatalf("GetDatabaseTables: %v", err)
	}

	names := make(map[string]DatabaseTableInfo)
	for _, tbl := range tables {
		names[tbl.TableName] = tbl
	}

	if _, ok := names["my_data"]; !ok {
		t.Fatalf("expected my_data in results, got %v", tables)
	}
	if !names["my_data"].HasConfiguration {
		t.Fatal("expected my_data to have configuration")
	}
	if !names["my_data"].ConfigurationName.Valid || names["my_data"].ConfigurationName.String != "my_data_cfg" {
		t.Fatalf("unexpected configuration name: %+v", names["my_data"].ConfigurationName)
	}
	if _, ok := names["other_data"]; !ok {
		t.Fatalf("expected other_data in results")
	}
	if names["other_data"].HasConfiguration {
		t.Fatal("expected other_data to have no configuration")
	}
}
