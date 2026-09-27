UPDATE rbac_roles SET name = 'viewer', description = 'Read-only access to granted tables and workflows (system role)', updated_at = NOW(3)
WHERE name = 'reader';

INSERT INTO rbac_permissions (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'workflow.view', 'View workflow hubs and their navigation members'),
(NOW(3), NOW(3), 'workflow.start', 'Use workflow actions (mutating workflow membership and related writes)'),
(NOW(3), NOW(3), 'table.view', 'View table data and structure for granted tables'),
(NOW(3), NOW(3), 'table.insert', 'Insert rows in granted tables'),
(NOW(3), NOW(3), 'table.edit', 'Edit rows in granted tables'),
(NOW(3), NOW(3), 'table.delete', 'Delete rows in granted tables'),
(NOW(3), NOW(3), 'dashboard.view', 'View granted dashboards')
ON DUPLICATE KEY UPDATE description = VALUES(description);

CREATE TABLE IF NOT EXISTS rbac_resource_grants (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_group_id BIGINT NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    resource_key VARCHAR(191) NOT NULL,
    can_view TINYINT(1) NOT NULL DEFAULT 0,
    can_insert TINYINT(1) NOT NULL DEFAULT 0,
    can_edit TINYINT(1) NOT NULL DEFAULT 0,
    can_delete TINYINT(1) NOT NULL DEFAULT 0,
    can_start TINYINT(1) NOT NULL DEFAULT 0,
    UNIQUE KEY uk_rbac_resource_grant (user_group_id, resource_type, resource_key),
    KEY idx_rbac_resource_grants_lookup (resource_type, resource_key)
);

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name IN ('workflow.view', 'table.view', 'dashboard.view')
WHERE r.name = 'viewer'
ON DUPLICATE KEY UPDATE role_id = role_id;

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name IN (
  'workflow.view', 'workflow.start', 'table.view', 'table.insert', 'table.edit', 'table.delete', 'dashboard.view'
) WHERE r.name = 'member'
ON DUPLICATE KEY UPDATE role_id = role_id;

INSERT INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'table', tc.name, 1, 1, 1, 1, 0
FROM user_groups g CROSS JOIN table_configurations tc
WHERE g.name = 'Everyone'
ON DUPLICATE KEY UPDATE can_view = VALUES(can_view);

INSERT INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'dashboard', td.name, 1, 0, 0, 0, 0
FROM user_groups g CROSS JOIN table_dashboards td
WHERE g.name = 'Everyone'
ON DUPLICATE KEY UPDATE can_view = VALUES(can_view);

INSERT INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'workflow', CAST(tw.id AS CHAR), 1, 0, 0, 0, 1
FROM user_groups g CROSS JOIN table_workflows tw
WHERE g.name = 'Everyone'
ON DUPLICATE KEY UPDATE can_view = VALUES(can_view);
