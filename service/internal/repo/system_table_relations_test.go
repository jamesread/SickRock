package repo

import (
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestEnsureSystemTableRelations(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = configureSQLiteDB(db); err != nil {
		t.Fatalf("configure: %v", err)
	}
	_, err = db.ExecContext(t.Context(), `
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
			pull_through_json TEXT
		);
		-- A backfilled relation (physical FK name) whose display name the user has edited.
		INSERT INTO table_relations (constraint_name, source_table_key, source_column, referenced_table_key, referenced_column, display_name)
		VALUES ('physical_fk_1', 'table_app_navigation_links', 'app_id', 'table_apps', 'id', 'My links');
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	r := NewRepository(db)
	for range 2 {
		if err := r.EnsureSystemTableRelations(t.Context()); err != nil {
			t.Fatalf("EnsureSystemTableRelations: %v", err)
		}
	}

	var rels []TableRelation
	if err := db.SelectContext(t.Context(), &rels, `
		SELECT id, constraint_name, source_table_key, source_column, referenced_table_key, referenced_column,
			on_delete_action, on_update_action, display_name, pull_through_json
		FROM table_relations ORDER BY source_column`); err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("got %d relations, want 2: %+v", len(rels), rels)
	}
	app, nav := rels[0], rels[1]
	if app.ConstraintName != "physical_fk_1" || app.DisplayName.String != "My links" {
		t.Errorf("existing app_id relation changed: %+v", app)
	}
	if app.PullThroughJSON.String != `[{"fk_column":"navigation_item_id","field":"name"}]` {
		t.Errorf("app_id pull-through = %q", app.PullThroughJSON.String)
	}
	if nav.ReferencedTableKey != "table_navigation" || nav.OnDeleteAction != "CASCADE" {
		t.Errorf("unexpected navigation_item_id relation: %+v", nav)
	}
}
