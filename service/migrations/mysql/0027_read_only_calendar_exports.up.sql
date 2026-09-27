CREATE TABLE IF NOT EXISTS read_only_calendar_exports (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    slug VARCHAR(191) NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    table_configuration VARCHAR(255) NOT NULL,
    table_view_id INT NULL,
    where_json TEXT NOT NULL DEFAULT '{}',
    weekends_only TINYINT(1) NOT NULL DEFAULT 0,
    display_mode VARCHAR(32) NOT NULL DEFAULT 'full',
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    allowed_group_ids TEXT NOT NULL DEFAULT '[]',
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_read_only_export_slug (slug)
);

INSERT INTO rbac_permissions (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'exports.manage', 'Create and edit read-only calendar export definitions')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'exports.manage'
WHERE r.name = 'superuser'
ON DUPLICATE KEY UPDATE role_id = role_id;
