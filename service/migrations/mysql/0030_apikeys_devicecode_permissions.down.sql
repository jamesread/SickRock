DELETE rp FROM rbac_role_permissions rp
INNER JOIN rbac_permissions p ON p.id = rp.permission_id
WHERE p.name IN ('apikeys.use', 'devicecode.claim');

DELETE FROM rbac_permissions WHERE name IN ('apikeys.use', 'devicecode.claim');
