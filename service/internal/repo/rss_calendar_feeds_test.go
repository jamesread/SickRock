package repo

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestGetRssCalendarFeedScansSQLiteTimestampString(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE rss_calendar_feeds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			feed_url TEXT NOT NULL,
			table_configuration TEXT NOT NULL,
			field_mapping_json TEXT NOT NULL DEFAULT '{}',
			unique_column TEXT NOT NULL DEFAULT '',
			refresh_interval_minutes INTEGER NOT NULL DEFAULT 60,
			enabled INTEGER NOT NULL DEFAULT 1,
			last_refresh_at TEXT,
			last_refresh_status TEXT NOT NULL DEFAULT '',
			last_refresh_message TEXT NOT NULL DEFAULT '',
			last_items_created INTEGER NOT NULL DEFAULT 0,
			last_items_updated INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO rss_calendar_feeds (
			name, feed_url, table_configuration, last_refresh_at, last_refresh_status
		) VALUES ('events', 'https://example.com/events.ics', 'calendar', '2026-10-04 18:00:00', 'ok');
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	feed, err := NewRepository(db).GetRssCalendarFeedByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	if !feed.LastRefreshAt.Valid {
		t.Fatal("expected last_refresh_at to scan")
	}
	if got := feed.LastRefreshAt.Time.Format("2006-01-02 15:04:05"); got != "2026-10-04 18:00:00" {
		t.Fatalf("last_refresh_at %s", got)
	}
	if feed.LastRefreshAt.Time.Location() != time.UTC && feed.LastRefreshAt.Time.Location() != time.Local {
		t.Fatalf("unexpected location %s", feed.LastRefreshAt.Time.Location())
	}
}
