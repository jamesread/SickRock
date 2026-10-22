-- Optional column each imported event records its source feed's id in, to trace events back to their feed.
ALTER TABLE rss_calendar_feeds
  ADD COLUMN IF NOT EXISTS feed_id_column VARCHAR(255) NOT NULL DEFAULT '';
