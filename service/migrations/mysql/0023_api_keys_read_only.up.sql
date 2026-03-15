-- Add read_only to table_api_keys (MySQL)
ALTER TABLE table_api_keys ADD COLUMN read_only BOOLEAN NOT NULL DEFAULT FALSE;
