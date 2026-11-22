package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jamesread/SickRock/internal/testdb"
)

func TestWithBulkEntryScanColumn(t *testing.T) {
	assert.Equal(t, map[string]any{"scan_column": "barcode"}, withBulkEntryScanColumn("", "barcode"))
	assert.Equal(t, map[string]any{"scan_column": "barcode"}, withBulkEntryScanColumn("not json", "barcode"))
	assert.Equal(t, map[string]any{"scan_column": "barcode"}, withBulkEntryScanColumn("[1]", "barcode"))
	assert.Equal(t,
		map[string]any{"scan_column": "barcode", "quantity_column": "qty", "save_on_scan": true},
		withBulkEntryScanColumn(`{"scan_column":"ean","quantity_column":"qty","save_on_scan":true}`, "barcode"))
}

func TestSetBulkEntryScanColumn(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `INSERT INTO table_configurations (name, title, db, "table") VALUES ('pantry', 'Pantry', 'main', 'pantry')`)
	require.NoError(t, err)
	r := NewRepository(db)

	require.NoError(t, r.SetBulkEntryScanColumn(t.Context(), "pantry", "barcode"))
	var raw string
	require.NoError(t, db.GetContext(t.Context(), &raw, `SELECT bulk_entry_json FROM table_configurations WHERE name = 'pantry'`))
	assert.JSONEq(t, `{"scan_column":"barcode"}`, raw)

	require.Error(t, r.SetBulkEntryScanColumn(t.Context(), "missing", "barcode"))
}
