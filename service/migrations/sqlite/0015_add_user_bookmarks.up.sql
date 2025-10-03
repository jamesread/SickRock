-- Add table_user_bookmarks table (SQLite)

CREATE TABLE IF NOT EXISTS table_user_bookmarks (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user INTEGER NOT NULL,
    navigation_item INTEGER NOT NULL
);
