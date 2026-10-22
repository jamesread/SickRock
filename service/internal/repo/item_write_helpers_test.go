package repo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestItemUpdateSet(t *testing.T) {
	columns := []FieldSpec{
		{Name: "name", Required: true},
		{Name: "sr_updated"},
		{Name: "Title", Required: true},
		{Name: "notes"},
		{Name: "workflow_id", Required: true},
	}

	t.Run("name, timestamp and fields", func(t *testing.T) {
		setParts, args := itemUpdateSet(columns, "Widget", map[string]string{"title": "Hello"}, "NOW()")
		assert.Equal(t, []string{"`name` = ?", "`sr_updated` = NOW()", "`title` = ?"}, setParts)
		assert.Equal(t, []any{"Widget", "Hello"}, args)
	})

	t.Run("empty nullable and navigation FKs become NULL", func(t *testing.T) {
		setParts, args := itemUpdateSet(columns[2:], "", map[string]string{"notes": "", "workflow_id": "0"}, "NOW()")
		assert.ElementsMatch(t, []string{"`notes` = NULL", "`workflow_id` = NULL"}, setParts)
		assert.Empty(t, args)
	})

	t.Run("empty required field is kept", func(t *testing.T) {
		setParts, args := itemUpdateSet(columns[2:3], "", map[string]string{"Title": ""}, "NOW()")
		assert.Equal(t, []string{"`Title` = ?"}, setParts)
		assert.Equal(t, []any{""}, args)
	})

	t.Run("unknown fields ignored", func(t *testing.T) {
		setParts, _ := itemUpdateSet(columns[3:4], "", map[string]string{"missing": "x"}, "NOW()")
		assert.Empty(t, setParts)
	})
}

func TestItemInsertColumns(t *testing.T) {
	columns := []FieldSpec{{Name: "sr_created"}, {Name: "sr_updated"}, {Name: "Title"}}

	insertColumns, placeholders, values := itemInsertColumns(columns, map[string]string{"title": "Hello", "missing": "x"}, "NOW()")

	assert.Equal(t, []string{"`sr_created`", "`sr_updated`", "`Title`"}, insertColumns)
	assert.Equal(t, []string{"NOW()", "NOW()", "?"}, placeholders)
	assert.Equal(t, []any{"Hello"}, values)
}

func TestWritableColumnPrefersExactMatch(t *testing.T) {
	columns := []FieldSpec{{Name: "NAME"}, {Name: "name"}}
	assert.Equal(t, "name", writableColumn(columns, "name").Name)
	assert.Equal(t, "NAME", writableColumn(columns, "Name").Name)
	assert.Nil(t, writableColumn(columns, "other"))
}

func TestConditionalFormattingRulesQuery(t *testing.T) {
	query, args := conditionalFormattingRulesQuery(true, "orders")
	assert.Contains(t, query, "is_active, sr_created, updated_at_unix")
	assert.Contains(t, query, "WHERE table_name = ?")
	assert.Equal(t, []any{"orders"}, args)

	query, args = conditionalFormattingRulesQuery(false, "")
	assert.NotContains(t, query, "sr_created")
	assert.NotContains(t, query, "WHERE")
	assert.Contains(t, query, "ORDER BY table_name ASC")
	assert.Empty(t, args)
}

func TestParseRuleTimestamp(t *testing.T) {
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	assert.True(t, want.Equal(parseRuleTimestamp("2026-01-02T03:04:05Z")))
	assert.True(t, want.Equal(parseRuleTimestamp("2026-01-02 03:04:05")))
	assert.True(t, want.Equal(parseRuleTimestamp("2026-01-02T03:04:05")))
	assert.True(t, parseRuleTimestamp("garbage").IsZero())
}
