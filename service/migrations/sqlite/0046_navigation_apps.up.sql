CREATE TABLE IF NOT EXISTS apps (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 99
);

CREATE TABLE IF NOT EXISTS app_navigation_links (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    app_id INTEGER NOT NULL,
    navigation_item_id INTEGER NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 99,
    UNIQUE (navigation_item_id),
    FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE,
    FOREIGN KEY (navigation_item_id) REFERENCES table_navigation(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_app_nav_links_app ON app_navigation_links (app_id);
