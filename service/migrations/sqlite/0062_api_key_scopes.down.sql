DROP TABLE IF EXISTS api_key_resources;
DROP TABLE IF EXISTS api_key_permissions;
ALTER TABLE api_keys DROP COLUMN restrict_resources;
ALTER TABLE api_keys DROP COLUMN restrict_permissions;
ALTER TABLE api_keys DROP COLUMN expires_at;
