INSERT INTO rbac_permissions (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'app.read', 'View application data (read-only APIs)'),
(NOW(3), NOW(3), 'exports.view', 'View read-only calendar exports when allowed by export configuration')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT INTO rbac_roles (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'reader', 'Read-only access to shared resources (system role)')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'app.read'
WHERE r.name = 'reader'
ON DUPLICATE KEY UPDATE role_id = role_id;

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'exports.view'
WHERE r.name = 'reader'
ON DUPLICATE KEY UPDATE role_id = role_id;

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'app.read'
WHERE r.name = 'member'
ON DUPLICATE KEY UPDATE role_id = role_id;

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'exports.view'
WHERE r.name = 'member'
ON DUPLICATE KEY UPDATE role_id = role_id;
