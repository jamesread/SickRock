package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestSetTableRowLabels(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err = db.ExecContext(t.Context(), `
		CREATE TABLE table_configurations (
			name TEXT PRIMARY KEY,
			db TEXT,
			"table" TEXT,
			title TEXT,
			ordinal INTEGER DEFAULT 0,
			create_button_text TEXT,
			row_name TEXT,
			row_name_plural TEXT,
			create_delegate TEXT,
			icon TEXT,
			primary_key_column TEXT,
			default_sort_column TEXT
		);
		INSERT INTO table_configurations (name, db, "table", title)
		VALUES ('quotes', 'main', 'quotes', 'Quotes');
	`); err != nil {
		t.Fatalf("setup schema: %v", err)
	}

	r := NewRepository(db)
	ctx := context.Background()

	if err = r.SetTableRowLabels(ctx, "quotes", "quote", "quotes"); err != nil {
		t.Fatalf("set labels: %v", err)
	}
	tc, err := r.GetTableConfiguration(ctx, "quotes")
	if err != nil {
		t.Fatalf("get tc: %v", err)
	}
	if tc.DisplayRowName() != "quote" {
		t.Fatalf("row name = %q, want quote", tc.DisplayRowName())
	}
	if tc.DisplayRowNamePlural() != "quotes" {
		t.Fatalf("row name plural = %q, want quotes", tc.DisplayRowNamePlural())
	}

	if err = r.SetTableRowLabels(ctx, "quotes", "", ""); err != nil {
		t.Fatalf("clear labels: %v", err)
	}
	tc, err = r.GetTableConfiguration(ctx, "quotes")
	if err != nil {
		t.Fatalf("get tc: %v", err)
	}
	if tc.DisplayRowName() != "Quotes" {
		t.Fatalf("fallback row name = %q, want Quotes", tc.DisplayRowName())
	}
}
