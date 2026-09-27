INSERT INTO rbac_permissions (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'apikeys.use', 'Create and manage your own API keys'),
(NOW(3), NOW(3), 'devicecode.claim', 'Claim device codes to sign in on other devices')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name IN ('apikeys.use', 'devicecode.claim')
WHERE r.name = 'member'
ON DUPLICATE KEY UPDATE role_id = role_id;
