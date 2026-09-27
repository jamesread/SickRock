CREATE TABLE IF NOT EXISTS rbac_table_group_roles (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_group_id BIGINT NOT NULL,
    table_key VARCHAR(191) NOT NULL,
    role_id BIGINT NOT NULL,
    UNIQUE KEY uk_table_group_role (user_group_id, table_key),
    KEY idx_table_group_roles_table (table_key),
    KEY idx_table_group_roles_role (role_id)
);

INSERT INTO rbac_table_group_roles (user_group_id, table_key, role_id)
SELECT g.user_group_id, g.resource_key,
       CASE
           WHEN g.can_insert != 0 OR g.can_edit != 0 OR g.can_delete != 0
               THEN (SELECT id FROM rbac_roles WHERE name = 'member' LIMIT 1)
           ELSE (SELECT id FROM rbac_roles WHERE name = 'viewer' LIMIT 1)
       END
FROM rbac_resource_grants g
WHERE g.resource_type = 'table'
ON DUPLICATE KEY UPDATE role_id = VALUES(role_id);

DELETE FROM rbac_resource_grants WHERE resource_type = 'table';
