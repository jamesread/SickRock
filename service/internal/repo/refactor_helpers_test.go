package repo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportWhereClause(t *testing.T) {
	clause, args := exportWhereClause(nil)
	assert.Empty(t, clause)
	assert.Nil(t, args)

	clause, args = exportWhereClause(map[string]string{"title": "%party%"})
	assert.Equal(t, " WHERE `title` LIKE ?", clause)
	assert.Equal(t, []any{"%party%"}, args)

	clause, _ = exportWhereClause(map[string]string{"status": "busy"})
	assert.Equal(t, " WHERE `status` = ?", clause)
}

func TestRowTimestamp(t *testing.T) {
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	assert.Equal(t, want, rowTimestamp(want))
	assert.True(t, want.Equal(rowTimestamp("2026-01-02 03:04:05")))
	assert.True(t, rowTimestamp("garbage").IsZero())
	assert.True(t, rowTimestamp(nil).IsZero())
	assert.True(t, rowTimestamp(42).IsZero())
}

func TestAddColumnSQLType(t *testing.T) {
	tests := []struct {
		field       FieldSpec
		isMySQL     bool
		wantType    string
		wantDefault string
	}{
		{FieldSpec{Type: "int64"}, false, "BIGINT", ""},
		{FieldSpec{Type: "user_ref"}, true, "BIGINT", ""},
		{FieldSpec{Type: "string"}, true, "TEXT", ""},
		{FieldSpec{Type: "tinyint"}, true, "TINYINT(1)", ""},
		{FieldSpec{Type: "tinyint"}, false, "INTEGER", ""},
		{FieldSpec{Type: "datetime"}, true, "DATETIME", ""},
		{FieldSpec{Type: "datetime", DefaultToCurrentTimestamp: true}, true, "DATETIME", " DEFAULT CURRENT_TIMESTAMP"},
		{FieldSpec{Type: "datetime", DefaultToCurrentTimestamp: true}, false, "DATETIME", " DEFAULT (datetime('now'))"},
		{FieldSpec{Type: "unknown"}, false, "TEXT", ""},
	}
	for _, tt := range tests {
		typ, def := addColumnSQLType(tt.field, tt.isMySQL)
		assert.Equal(t, tt.wantType, typ, "%+v mysql=%v", tt.field, tt.isMySQL)
		assert.Equal(t, tt.wantDefault, def, "%+v mysql=%v", tt.field, tt.isMySQL)
	}
}

func TestCheckExprPlaceholder(t *testing.T) {
	scopes := []relationExprScope{{
		aliases: []string{"orders"},
		columns: map[string]struct{}{"qty": {}},
	}}
	require.NoError(t, checkExprPlaceholder("orders", "QTY", scopes))
	require.ErrorContains(t, checkExprPlaceholder("orders", "price", scopes), "unknown column")
	require.ErrorContains(t, checkExprPlaceholder("customers", "qty", scopes), "unknown table")
}

func TestValidateCalculatedExpr(t *testing.T) {
	scopes := []relationExprScope{{
		aliases: []string{"orders"},
		columns: map[string]struct{}{"qty": {}, "price": {}},
	}}
	require.NoError(t, validateCalculatedExpr("{orders.qty} * {orders.price}", scopes))
	require.ErrorContains(t, validateCalculatedExpr("", scopes), "required")
	require.ErrorContains(t, validateCalculatedExpr("1 + 1", scopes), "placeholders")
	require.ErrorContains(t, validateCalculatedExpr("{orders.qty} *", scopes), "invalid expr")
}

func TestWithCalculatedColumns(t *testing.T) {
	item := Item{ID: "1", Fields: map[string]any{"qty": 2}}
	columns := []RelationPullThroughField{
		{FKColumn: "customer_id", Field: "name"},
		{DisplayLabel: "Total", Expr: "{orders.qty} * {orders.price}"},
		{DisplayLabel: "Broken", Expr: "{orders.qty} *"},
	}
	env := map[string]any{"orders": map[string]any{"qty": 2.0, "price": 3.0}}

	got := withCalculatedColumns(item, columns, env)

	require.Contains(t, got.Fields, RelationCalcStorageKey("Total", 1))
	assert.NotEmpty(t, got.Fields[RelationCalcStorageKey("Total", 1)])
	assert.Empty(t, got.Fields[RelationCalcStorageKey("Broken", 2)])
	assert.Len(t, item.Fields, 1, "input item is not mutated")
}
