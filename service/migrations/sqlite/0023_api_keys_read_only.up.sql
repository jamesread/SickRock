-- Add read_only to table_api_keys (SQLite)
ALTER TABLE table_api_keys ADD COLUMN read_only BOOLEAN NOT NULL DEFAULT 0;
