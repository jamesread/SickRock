package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestUpdateTableRelationDisplayName(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if _, err = db.ExecContext(ctx, `
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
		) VALUES ('fk_test', 'orders', 'customer_id', 'customers', 'id');
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	repo := NewRepository(db)
	if err = repo.UpdateTableRelation(ctx, "fk_test", "Customer orders", nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	rel, err := repo.GetTableRelationByConstraintName(ctx, "fk_test")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !rel.DisplayName.Valid || rel.DisplayName.String != "Customer orders" {
		t.Fatalf("display name: %+v", rel.DisplayName)
	}
}
