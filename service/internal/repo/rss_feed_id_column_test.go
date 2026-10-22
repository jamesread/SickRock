package repo

import (
	"context"
	"fmt"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jamesread/SickRock/internal/rsscalendar"
	"github.com/jamesread/SickRock/internal/testdb"
)

func TestRssFeedItemFields(t *testing.T) {
	mappings := []rsscalendar.FieldMapping{{RssField: "title", Column: "name"}}
	values := map[string]string{"title": "Party"}

	assert.Equal(t, map[string]string{"name": "Party"}, rssFeedItemFields(&RssCalendarFeed{ID: 7}, values, mappings))
	assert.Equal(t, map[string]string{"name": "Party", "source_feed": "7"},
		rssFeedItemFields(&RssCalendarFeed{ID: 7, FeedIDColumn: " source_feed "}, values, mappings))
	assert.Empty(t, rssFeedItemFields(&RssCalendarFeed{ID: 7, FeedIDColumn: "source_feed"}, map[string]string{}, mappings),
		"items with no mapped values are still skipped")
}

func TestSyncRssFeedItemsRecordsFeedID(t *testing.T) {
	ctx := context.Background()
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(ctx, `
		INSERT INTO table_configurations (name, db, "table", title) VALUES ('events', 'main', 'events', 'Events');
		CREATE TABLE events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			guid TEXT,
			source_feed TEXT,
			sr_created TEXT,
			sr_updated TEXT
		);`)
	require.NoError(t, err)
	r := NewRepository(db)

	feedID, err := r.CreateRssCalendarFeed(ctx, &RssCalendarFeed{
		Name: "Events", FeedURL: "https://example.com/feed", TableConfiguration: "events",
		UniqueColumn: "guid", FeedIDColumn: "source_feed",
	})
	require.NoError(t, err)
	feed, err := r.GetRssCalendarFeedByID(ctx, feedID)
	require.NoError(t, err)
	assert.Equal(t, "source_feed", feed.FeedIDColumn, "feed ID column round-trips")

	mappings := []rsscalendar.FieldMapping{{RssField: "title", Column: "name"}, {RssField: "guid", Column: "guid"}}
	items := []*gofeed.Item{{Title: "Party", GUID: "g1"}}

	result, err := r.syncRssFeedItems(ctx, feed, mappings, items)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)

	result, err = r.syncRssFeedItems(ctx, feed, mappings, items)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Updated, "unique column still matches the existing row")

	rows, err := r.ListItemsInTable(ctx, "events", nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, feed.ID, intField(t, rows[0].Fields["source_feed"]))
}

func intField(t *testing.T, v any) int {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return int(n)
	case string:
		var out int
		_, err := fmt.Sscan(n, &out)
		require.NoError(t, err)
		return out
	}
	t.Fatalf("unexpected field type %T", v)
	return 0
}
