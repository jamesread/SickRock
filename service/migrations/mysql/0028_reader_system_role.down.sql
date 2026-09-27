DELETE FROM rbac_role_permissions
WHERE role_id IN (SELECT id FROM rbac_roles WHERE name = 'reader');

DELETE FROM rbac_roles WHERE name = 'reader';

DELETE FROM rbac_role_permissions
WHERE permission_id IN (SELECT id FROM rbac_permissions WHERE name IN ('app.read', 'exports.view'))
  AND role_id IN (SELECT id FROM rbac_roles WHERE name = 'member');

DELETE FROM rbac_permissions WHERE name IN ('app.read', 'exports.view');
