-- Remove read_only from table_api_keys (MySQL)
ALTER TABLE table_api_keys DROP COLUMN read_only;
