package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarExportColumnAllowlist(t *testing.T) {
	cols := []FieldSpec{{Name: "title"}, {Name: "starts"}, {Name: "id"}, {Name: "notes"}, {Name: "sr_created"}}

	assert.Equal(t, []string{"id", "sr_created", "starts"}, calendarExportColumnAllowlist(true, cols, "id"))
	assert.Equal(t, []string{"id", "sr_created", "starts", "title", "notes"}, calendarExportColumnAllowlist(false, cols, "id"))
	assert.Equal(t, []string{"id", "sr_created", "starts"}, calendarExportColumnAllowlist(true, cols, ""), "empty key defaults to id")
}

func TestGroupFieldGroupRows(t *testing.T) {
	rows := []TableFieldGroup{
		{ID: 5, Ordinal: 1, Title: "Main", ColumnName: "a"},
		{ID: 5, Ordinal: 1, Title: "Main", ColumnName: "b"},
		{ID: 2, Ordinal: 2, Title: "Empty"},
	}
	assert.Equal(t, []FieldGroupInput{
		{Ordinal: 1, Title: "Main", ColumnNames: []string{"a", "b"}},
		{Ordinal: 2, Title: "Empty"},
	}, groupFieldGroupRows(rows))
	assert.Empty(t, groupFieldGroupRows(nil))
}

func TestValidatePageLayoutBlocks(t *testing.T) {
	allowed := map[string]struct{}{"fk_a": {}, "fk_b": {}}
	relation := func(name string) PageLayoutBlockInput {
		return PageLayoutBlockInput{BlockType: PageBlockTypeRelation, ConfigJSON: `{"constraint_name":"` + name + `"}`}
	}
	quickJump := PageLayoutBlockInput{BlockType: PageBlockTypeQuickJump}

	require.NoError(t, validatePageLayoutBlocks([]PageLayoutBlockInput{relation("fk_a"), quickJump, relation("fk_b")}, allowed))
	require.ErrorContains(t, validatePageLayoutBlocks([]PageLayoutBlockInput{quickJump, quickJump}, allowed), "only one quick_jump")
	require.ErrorContains(t, validatePageLayoutBlocks([]PageLayoutBlockInput{relation("fk_a"), relation("fk_a")}, allowed), "more than one block")
	require.ErrorContains(t, validatePageLayoutBlocks([]PageLayoutBlockInput{relation("fk_z")}, allowed), "unknown constraint")
	require.ErrorContains(t, validatePageLayoutBlocks([]PageLayoutBlockInput{{BlockType: ""}}, allowed), "block_type is required")
}

func TestCanonicalAllowedColumn(t *testing.T) {
	got, err := canonicalAllowedColumn(map[string]struct{}{"Title": {}}, "title")
	require.NoError(t, err)
	assert.Equal(t, "Title", got)

	got, err = canonicalAllowedColumn(nil, "anything")
	require.NoError(t, err)
	assert.Equal(t, "anything", got)

	_, err = canonicalAllowedColumn(map[string]struct{}{"Title": {}}, "missing")
	require.Error(t, err)
}

func TestApiKeyUpdateSet(t *testing.T) {
	updates, args := apiKeyUpdateSet(nil, nil, nil)
	assert.Empty(t, updates)
	assert.Empty(t, args)

	name, readOnly := "ci", true
	updates, args = apiKeyUpdateSet(&name, nil, &readOnly)
	assert.Equal(t, []string{"name = ?", "read_only = ?"}, updates)
	assert.Equal(t, []any{"ci", 1}, args)
}

func TestRelationSyntheticColumnKey(t *testing.T) {
	assert.Equal(t, PullThroughStorageKey("customer_id", "name"), relationSyntheticColumnKey(RelationPullThroughField{FKColumn: "customer_id", Field: "name"}, 0))
	assert.Equal(t, RelationCalcStorageKey("Total", 2), relationSyntheticColumnKey(RelationPullThroughField{DisplayLabel: "Total", Expr: "{a.b}"}, 2))
	assert.Empty(t, relationSyntheticColumnKey(RelationPullThroughField{}, 0))
}

func TestNormalizeRssCalendarFeed(t *testing.T) {
	feed := &RssCalendarFeed{Name: " Events ", FeedURL: " https://example.com/feed ", TableConfiguration: "events"}
	require.NoError(t, normalizeRssCalendarFeed(feed))
	assert.Equal(t, "Events", feed.Name)
	assert.Equal(t, 60, feed.RefreshIntervalMinutes)
	assert.Equal(t, "{}", feed.FieldMappingJSON)

	require.Error(t, normalizeRssCalendarFeed(&RssCalendarFeed{Name: "x"}))
}
