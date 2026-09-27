CREATE TABLE IF NOT EXISTS rbac_table_group_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_group_id INTEGER NOT NULL,
    table_key TEXT NOT NULL,
    role_id INTEGER NOT NULL,
    UNIQUE(user_group_id, table_key)
);

CREATE INDEX IF NOT EXISTS idx_table_group_roles_table ON rbac_table_group_roles(table_key);
CREATE INDEX IF NOT EXISTS idx_table_group_roles_role ON rbac_table_group_roles(role_id);

INSERT OR IGNORE INTO rbac_table_group_roles (user_group_id, table_key, role_id)
SELECT g.user_group_id, g.resource_key,
       CASE
           WHEN g.can_insert != 0 OR g.can_edit != 0 OR g.can_delete != 0
               THEN (SELECT id FROM rbac_roles WHERE name = 'member' LIMIT 1)
           ELSE (SELECT id FROM rbac_roles WHERE name = 'viewer' LIMIT 1)
       END
FROM rbac_resource_grants g
WHERE g.resource_type = 'table';

DELETE FROM rbac_resource_grants WHERE resource_type = 'table';
