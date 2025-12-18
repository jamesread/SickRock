-- Revert: Move view type back from table_views to table_configurations (SQLite)

-- Add view column back to table_configurations
-- SQLite doesn't support ADD COLUMN with default in all versions, so we recreate
CREATE TABLE table_configurations_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    title TEXT,
    ordinal INTEGER DEFAULT 0,
    db TEXT,
    create_button_text TEXT,
    icon TEXT,
    view TEXT,
    `table` TEXT
);

INSERT INTO table_configurations_new (id, name, title, ordinal, db, create_button_text, icon, `table`)
SELECT id, name, title, ordinal, db, create_button_text, icon, `table`
FROM table_configurations;

DROP TABLE table_configurations;
ALTER TABLE table_configurations_new RENAME TO table_configurations;

-- Migrate view_type back to table_configurations.view
-- Set view to 'calendar' where default view has view_type = 'calendar'
UPDATE table_configurations
SET view = 'calendar'
WHERE name IN (
  SELECT tv.table_name
  FROM table_views tv
  WHERE tv.view_type = 'calendar' AND tv.is_default = 1
);

-- Set default to 'table' for others
UPDATE table_configurations
SET view = 'table'
WHERE view IS NULL;

-- Remove view_type column from table_views
-- SQLite doesn't support DROP COLUMN directly
CREATE TABLE table_views_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    view_name TEXT NOT NULL,
    is_default INTEGER DEFAULT 0,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(table_name, view_name)
);

INSERT INTO table_views_new (id, table_name, view_name, is_default, sr_created, updated_at_unix)
SELECT id, table_name, view_name, is_default, sr_created, updated_at_unix
FROM table_views;

DROP TABLE table_views;
ALTER TABLE table_views_new RENAME TO table_views;

-- Recreate foreign key constraint
CREATE TABLE table_view_columns_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    view_id INTEGER NOT NULL,
    column_name TEXT NOT NULL,
    is_visible INTEGER DEFAULT 1,
    column_order INTEGER DEFAULT 0,
    column_width INTEGER,
    sort_order TEXT,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (view_id) REFERENCES table_views(id) ON DELETE CASCADE,
    UNIQUE(view_id, column_name)
);

INSERT INTO table_view_columns_new
SELECT * FROM table_view_columns;

DROP TABLE table_view_columns;
ALTER TABLE table_view_columns_new RENAME TO table_view_columns;
