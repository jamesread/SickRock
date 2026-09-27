DELETE FROM rbac_table_group_roles WHERE table_key = 'table_logs';

DELETE rp FROM rbac_role_permissions rp
INNER JOIN rbac_roles r ON r.id = rp.role_id
INNER JOIN rbac_permissions p ON p.id = rp.permission_id
WHERE r.name = 'member' AND p.name = 'audit.view';

INSERT INTO rbac_roles (name, description, created_at, updated_at)
VALUES ('auditor', 'View security and audit logs (system role)', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'auditor';

INSERT IGNORE INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g JOIN rbac_roles r ON r.name = 'auditor'
WHERE g.name = 'Administrators';
