DELETE FROM rbac_role_permissions WHERE permission_id IN (
  SELECT id FROM rbac_permissions WHERE name IN ('apikeys.use', 'devicecode.claim')
);
DELETE FROM rbac_permissions WHERE name IN ('apikeys.use', 'devicecode.claim');
