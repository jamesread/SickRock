INSERT OR IGNORE INTO rbac_permissions (name, description) VALUES
('apikeys.use', 'Create and manage your own API keys'),
('devicecode.claim', 'Claim device codes to sign in on other devices');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name IN ('apikeys.use', 'devicecode.claim')
WHERE r.name = 'member';
