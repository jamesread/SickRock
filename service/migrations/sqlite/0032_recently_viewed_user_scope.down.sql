CREATE TABLE table_recently_viewed_legacy (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    table_id INTEGER NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now'))
);

DROP TABLE table_recently_viewed;
ALTER TABLE table_recently_viewed_legacy RENAME TO table_recently_viewed;
