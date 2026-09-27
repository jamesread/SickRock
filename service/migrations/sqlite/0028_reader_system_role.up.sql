INSERT OR IGNORE INTO rbac_permissions (name, description) VALUES
('app.read', 'View application data (read-only APIs)'),
('exports.view', 'View read-only calendar exports when allowed by export configuration');

INSERT OR IGNORE INTO rbac_roles (name, description) VALUES
('reader', 'Read-only access to shared resources (system role)');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'app.read'
WHERE r.name = 'reader';

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'exports.view'
WHERE r.name = 'reader';

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'app.read'
WHERE r.name = 'member';

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'exports.view'
WHERE r.name = 'member';
