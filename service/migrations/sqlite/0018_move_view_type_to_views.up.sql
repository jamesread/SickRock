-- Move view type from table_configurations to table_views (SQLite)
-- Add view_type column to table_views with default 'table'
ALTER TABLE table_views
  ADD COLUMN view_type TEXT NOT NULL DEFAULT 'table';

-- Migrate existing calendar views from table_configurations to table_views
-- Set view_type to 'calendar' for default views where table_configurations.view = 'calendar'
UPDATE table_views
SET view_type = 'calendar'
WHERE id IN (
  SELECT tv.id
  FROM table_views tv
  INNER JOIN table_configurations tc ON tv.table_name = tc.name
  WHERE tc.view = 'calendar' AND tv.is_default = 1
);

-- Remove view column from table_configurations
-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
CREATE TABLE table_configurations_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    title TEXT,
    ordinal INTEGER DEFAULT 0,
    db TEXT,
    create_button_text TEXT,
    icon TEXT,
    `table` TEXT
);

INSERT INTO table_configurations_new (id, name, title, ordinal, db, create_button_text, icon, `table`)
SELECT id, name, title, ordinal, db, create_button_text, icon, `table`
FROM table_configurations;

DROP TABLE table_configurations;
ALTER TABLE table_configurations_new RENAME TO table_configurations;
