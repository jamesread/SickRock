package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func setupFieldGroupsTestDB(t *testing.T) *Repository {
	t.Helper()
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE table_field_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_configuration TEXT NOT NULL,
			ordinal INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL
		);
		CREATE TABLE table_field_group_columns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			field_group_id INTEGER NOT NULL,
			column_name TEXT NOT NULL,
			ordinal INTEGER NOT NULL DEFAULT 0,
			UNIQUE (field_group_id, column_name),
			FOREIGN KEY (field_group_id) REFERENCES table_field_groups(id) ON DELETE CASCADE
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return NewRepository(db)
}

func TestGetItemFieldGroupsLoad(t *testing.T) {
	repo := setupFieldGroupsTestDB(t)
	ctx := context.Background()
	db := repo.db
	res, err := db.ExecContext(ctx, `
		INSERT INTO table_field_groups (table_configuration, ordinal, title) VALUES ('orders', 1, 'Main')`)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	gid, _ := res.LastInsertId()
	if _, err = db.ExecContext(ctx, `
		INSERT INTO table_field_group_columns (field_group_id, column_name, ordinal) VALUES (?, 'id', 1), (?, 'name', 2)`,
		gid, gid); err != nil {
		t.Fatalf("insert cols: %v", err)
	}
	groups, err := repo.GetItemFieldGroups(ctx, "orders")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(groups) != 1 || groups[0].Title != "Main" || len(groups[0].ColumnNames) != 2 {
		t.Fatalf("groups: %+v", groups)
	}
}

func TestValidateFieldGroups(t *testing.T) {
	allowed := map[string]struct{}{"id": {}, "name": {}, "status": {}}
	if err := validateFieldGroups([]FieldGroupInput{
		{Ordinal: 1, Title: "Main", ColumnNames: []string{"id", "name"}},
	}, allowed); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := validateFieldGroups([]FieldGroupInput{
		{Ordinal: 1, Title: "X", ColumnNames: []string{"missing"}},
	}, allowed); err == nil {
		t.Fatal("expected unknown column")
	}
	if err := validateFieldGroups([]FieldGroupInput{
		{Ordinal: 1, Title: "A", ColumnNames: []string{"id"}},
		{Ordinal: 2, Title: "B", ColumnNames: []string{"id"}},
	}, allowed); err == nil {
		t.Fatal("expected duplicate column")
	}
	if err := validateFieldGroups([]FieldGroupInput{
		{Ordinal: 1, Title: "", ColumnNames: []string{"id"}},
	}, allowed); err == nil {
		t.Fatal("expected title required")
	}
}
