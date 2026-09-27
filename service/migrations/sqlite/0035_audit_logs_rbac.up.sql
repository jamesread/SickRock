-- Audit logs: RBAC permission only (not table ACL grants).
DELETE FROM rbac_table_group_roles WHERE table_key = 'table_logs';

DELETE FROM rbac_role_permissions
WHERE role_id IN (SELECT id FROM rbac_roles WHERE name = 'member')
  AND permission_id IN (SELECT id FROM rbac_permissions WHERE name = 'audit.view');

INSERT OR IGNORE INTO rbac_roles (name, description) VALUES
('auditor', 'View security and audit logs (system role)');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'auditor';

INSERT OR IGNORE INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g JOIN rbac_roles r ON r.name = 'auditor'
WHERE g.name = 'Administrators';
