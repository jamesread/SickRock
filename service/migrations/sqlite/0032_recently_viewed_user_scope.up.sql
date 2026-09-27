DELETE FROM table_recently_viewed;

CREATE TABLE table_recently_viewed_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    table_id TEXT NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(user_account_id, name, table_id)
);

DROP TABLE table_recently_viewed;
ALTER TABLE table_recently_viewed_new RENAME TO table_recently_viewed;

CREATE INDEX IF NOT EXISTS idx_recently_viewed_user_updated ON table_recently_viewed(user_account_id, updated_at_unix DESC);
