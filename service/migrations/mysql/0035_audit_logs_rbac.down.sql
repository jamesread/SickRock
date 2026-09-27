DELETE gr FROM rbac_group_roles gr
INNER JOIN user_groups g ON g.id = gr.user_group_id
INNER JOIN rbac_roles r ON r.id = gr.role_id
WHERE g.name = 'Administrators' AND r.name = 'auditor';

DELETE rp FROM rbac_role_permissions rp INNER JOIN rbac_roles r ON r.id = rp.role_id WHERE r.name = 'auditor';
DELETE FROM rbac_roles WHERE name = 'auditor';

INSERT IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'member';

INSERT INTO rbac_table_group_roles (user_group_id, table_key, role_id)
SELECT g.id, 'table_logs', r.id
FROM user_groups g JOIN rbac_roles r ON r.name = 'member'
WHERE g.name = 'Administrators'
ON DUPLICATE KEY UPDATE role_id = VALUES(role_id);
