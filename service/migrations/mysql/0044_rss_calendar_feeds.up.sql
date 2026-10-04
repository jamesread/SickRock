CREATE TABLE IF NOT EXISTS rss_calendar_feeds (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    feed_url TEXT NOT NULL,
    table_configuration VARCHAR(255) NOT NULL,
    field_mapping_json TEXT NOT NULL,
    unique_column VARCHAR(255) NOT NULL DEFAULT '',
    refresh_interval_minutes INT NOT NULL DEFAULT 60,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    last_refresh_at DATETIME(3) NULL,
    last_refresh_status VARCHAR(64) NOT NULL DEFAULT '',
    last_refresh_message TEXT NOT NULL,
    last_items_created INT NOT NULL DEFAULT 0,
    last_items_updated INT NOT NULL DEFAULT 0,
    sr_created DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    sr_updated DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    KEY idx_rss_calendar_feeds_enabled (enabled)
);
