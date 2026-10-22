package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestSetColumnDisplaySettingsUpsertAndClear(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = configureSQLiteDB(db); err != nil {
		t.Fatalf("configure: %v", err)
	}
	_, err = db.ExecContext(t.Context(), `
		CREATE TABLE table_column_display (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_configuration TEXT NOT NULL,
			column_name TEXT NOT NULL,
			display_label TEXT,
			value_prefix TEXT,
			value_suffix TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE (table_configuration, column_name)
		);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	r := NewRepository(db)
	ctx := context.Background()
	allowed := map[string]struct{}{"amount": {}, "currency": {}}

	if err = r.SetColumnDisplaySettings(ctx, "orders", []ColumnDisplayInput{
		{ColumnName: "amount", DisplayLabel: "Total", ValuePrefix: "{currency} ", ValueSuffix: ""},
	}, allowed); err != nil {
		t.Fatalf("set: %v", err)
	}
	m, err := r.GetColumnDisplaySettings(ctx, "orders")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m["amount"].DisplayLabel.String != "Total" || m["amount"].ValuePrefix.String != "{currency} " {
		t.Fatalf("unexpected row: %+v", m["amount"])
	}

	if err = r.SetColumnDisplaySettings(ctx, "orders", []ColumnDisplayInput{
		{ColumnName: "amount", DisplayLabel: "", ValuePrefix: "", ValueSuffix: ""},
	}, allowed); err != nil {
		t.Fatalf("clear: %v", err)
	}
	m, err = r.GetColumnDisplaySettings(ctx, "orders")
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %+v", m)
	}
}
