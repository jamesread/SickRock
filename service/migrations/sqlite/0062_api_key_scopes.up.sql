-- API key scopes: a key may be limited to some of its owner's permissions (api_key_permissions)
-- and to some tables, dashboards and workflows (api_key_resources), and may expire. The owner's
-- current access is always the ceiling.
ALTER TABLE api_keys ADD COLUMN expires_at BIGINT;
ALTER TABLE api_keys ADD COLUMN restrict_permissions INTEGER NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN restrict_resources INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS api_key_permissions (
    api_key_id INTEGER NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    permission_name TEXT NOT NULL,
    PRIMARY KEY (api_key_id, permission_name)
);

CREATE TABLE IF NOT EXISTS api_key_resources (
    api_key_id INTEGER NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL,
    resource_key TEXT NOT NULL,
    PRIMARY KEY (api_key_id, resource_type, resource_key)
);
