package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jamesread/SickRock/internal/repo"
)

func TestItemFieldsToStrings(t *testing.T) {
	fields := map[string]any{
		"name":    "Widget",
		"count":   3,
		"created": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		"notes":   nil,
	}

	assert.Equal(t, map[string]string{
		"name":    "Widget",
		"count":   "3",
		"created": "2026-01-02 03:04:05",
	}, itemFieldsToStrings(fields, false))

	withNulls := itemFieldsToStrings(fields, true)
	assert.Contains(t, withNulls, "notes")
	assert.Empty(t, withNulls["notes"])
}

func TestMarkdownRuleApplies(t *testing.T) {
	tests := []struct {
		conditionType  string
		conditionValue string
		fieldValue     string
		want           bool
	}{
		{"always", "", "anything", true},
		{"equals", "abc", "abc", true},
		{"equals", "abc", "ABC", false},
		{"contains", "OO", "foobar", true},
		{"greater_than", "10", "11", true},
		{"greater_than", "10", "10", false},
		{"greater_than", "10", "not a number", false},
		{"less_than", "10", "9.5", true},
		{"less_than", "x", "1", false},
		{"unknown", "", "", false},
	}
	for _, tt := range tests {
		rule := &repo.ConditionalFormattingRule{ConditionType: tt.conditionType, ConditionValue: tt.conditionValue}
		assert.Equal(t, tt.want, markdownRuleApplies(rule, tt.fieldValue), "%s %q vs %q", tt.conditionType, tt.conditionValue, tt.fieldValue)
	}
}

func TestApplyMarkdownRules(t *testing.T) {
	rules := []*repo.ConditionalFormattingRule{
		{ColumnName: "body", FormatType: "markdown", IsActive: true, ConditionType: "always"},
		{ColumnName: "inactive", FormatType: "markdown", IsActive: false, ConditionType: "always"},
		{ColumnName: "color", FormatType: "background", IsActive: true, ConditionType: "always"},
	}
	fields := map[string]any{"body": "**hi**", "inactive": "x", "color": "red"}
	out := map[string]string{}

	applyMarkdownRules(rules, fields, out)

	assert.Contains(t, out, "bodyMarkdown")
	assert.Contains(t, out["bodyMarkdown"], "<strong>hi</strong>")
	assert.NotContains(t, out, "inactiveMarkdown")
	assert.NotContains(t, out, "colorMarkdown")
}

func TestPickDefaultTableView(t *testing.T) {
	views := []repo.TableView{{ID: 1}, {ID: 2, IsDefault: true}, {ID: 3}}

	assert.Equal(t, 3, pickDefaultTableView(views, 3).ID, "user default wins")
	assert.Equal(t, 2, pickDefaultTableView(views, 0).ID, "table default when no user default")
	assert.Equal(t, 2, pickDefaultTableView(views, 99).ID, "table default when user default is missing")
	assert.Equal(t, 1, pickDefaultTableView([]repo.TableView{{ID: 1}, {ID: 4}}, 0).ID, "first view otherwise")
}

func TestIsWorkflowHubLink(t *testing.T) {
	item := repo.NavigationItem{}
	item.WorkflowID.Valid, item.WorkflowID.Int64 = true, 7

	assert.True(t, isWorkflowHubLink(item, 7))
	assert.False(t, isWorkflowHubLink(item, 8))

	item.TableName.Valid, item.TableName.String = true, "orders"
	assert.False(t, isWorkflowHubLink(item, 7), "a table link is not the hub")
}

func TestToInt32(t *testing.T) {
	assert.Equal(t, int32(42), toInt32(42))
	assert.Equal(t, int32(2147483647), toInt32(int64(1)<<40))
	assert.Equal(t, int32(-2147483648), toInt32(-(int64(1) << 40)))
}
