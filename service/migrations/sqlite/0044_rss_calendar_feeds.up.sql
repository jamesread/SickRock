CREATE TABLE IF NOT EXISTS rss_calendar_feeds (
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
    last_items_updated INTEGER NOT NULL DEFAULT 0,
    sr_created TEXT DEFAULT (datetime('now')),
    sr_updated TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_rss_calendar_feeds_enabled
    ON rss_calendar_feeds (enabled);
