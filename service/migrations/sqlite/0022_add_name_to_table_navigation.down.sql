-- Remove name column from table_navigation (SQLite)

-- SQLite 3.35+ supports DROP COLUMN
ALTER TABLE table_navigation DROP COLUMN name;
