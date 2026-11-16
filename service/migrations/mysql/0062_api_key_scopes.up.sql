-- API key scopes: a key may be limited to some of its owner's permissions (api_key_permissions)
-- and to some tables, dashboards and workflows (api_key_resources), and may expire. The owner's
-- current access is always the ceiling.
ALTER TABLE api_keys
    ADD COLUMN expires_at BIGINT NULL,
    ADD COLUMN restrict_permissions TINYINT(1) NOT NULL DEFAULT 0,
    ADD COLUMN restrict_resources TINYINT(1) NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS api_key_permissions (
    api_key_id INT UNSIGNED NOT NULL,
    permission_name VARCHAR(128) NOT NULL,
    PRIMARY KEY (api_key_id, permission_name),
    CONSTRAINT fk_api_key_permissions_key FOREIGN KEY (api_key_id) REFERENCES api_keys (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS api_key_resources (
    api_key_id INT UNSIGNED NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    resource_key VARCHAR(255) NOT NULL,
    PRIMARY KEY (api_key_id, resource_type, resource_key),
    CONSTRAINT fk_api_key_resources_key FOREIGN KEY (api_key_id) REFERENCES api_keys (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
