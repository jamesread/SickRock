-- Add initial_route column to table_users (SQLite)

ALTER TABLE table_users
  ADD COLUMN initial_route TEXT DEFAULT '/';
