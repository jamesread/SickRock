DELETE FROM rbac_group_roles
WHERE user_group_id IN (SELECT id FROM user_groups WHERE name = 'Administrators')
  AND role_id IN (SELECT id FROM rbac_roles WHERE name = 'auditor');

DELETE FROM rbac_role_permissions WHERE role_id IN (SELECT id FROM rbac_roles WHERE name = 'auditor');
DELETE FROM rbac_roles WHERE name = 'auditor';

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'member';

INSERT OR IGNORE INTO rbac_table_group_roles (user_group_id, table_key, role_id)
SELECT g.id, 'table_logs', r.id
FROM user_groups g JOIN rbac_roles r ON r.name = 'member'
WHERE g.name = 'Administrators';
