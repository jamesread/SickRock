CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    sr_created DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    event_type VARCHAR(64) NOT NULL,
    message VARCHAR(512) NOT NULL DEFAULT '',
    actor_user_id BIGINT NULL,
    actor_username VARCHAR(191) NOT NULL DEFAULT '',
    related_user VARCHAR(191) NOT NULL DEFAULT '',
    related_tc VARCHAR(191) NOT NULL DEFAULT '',
    related_row_id VARCHAR(191) NOT NULL DEFAULT '',
    ip_address VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    success TINYINT NOT NULL DEFAULT 1,
    details_json TEXT NOT NULL,
    KEY idx_audit_logs_sr_created (sr_created),
    KEY idx_audit_logs_event_type (event_type),
    KEY idx_audit_logs_related_tc (related_tc)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO rbac_permissions (name, description, created_at, updated_at)
VALUES ('audit.view', 'View security and audit logs', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'superuser';

INSERT INTO rbac_roles (name, description, created_at, updated_at)
VALUES ('auditor', 'View security and audit logs (system role)', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'auditor';

INSERT IGNORE INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g JOIN rbac_roles r ON r.name = 'auditor'
WHERE g.name = 'Administrators';

INSERT INTO table_configurations (name, title, `db`, `table`, ordinal, icon)
VALUES ('table_logs', 'Audit logs', 'main', 'audit_logs', 6, 'ShieldIcon')
ON DUPLICATE KEY UPDATE title = VALUES(title), `table` = VALUES(`table`), ordinal = VALUES(ordinal);

INSERT INTO table_views (table_name, view_name, is_default, view_type)
VALUES ('table_logs', 'Audit log', 1, 'table')
ON DUPLICATE KEY UPDATE is_default = VALUES(is_default);

INSERT INTO table_view_columns (view_id, column_name, is_visible, column_order, sort_order)
SELECT tv.id, cols.name, 1, cols.ord, IF(cols.name = 'sr_created', 'desc', '')
FROM table_views tv
JOIN (
    SELECT 'sr_created' AS name, 0 AS ord UNION ALL
    SELECT 'event_type', 10 UNION ALL
    SELECT 'message', 20 UNION ALL
    SELECT 'actor_username', 30 UNION ALL
    SELECT 'related_user', 40 UNION ALL
    SELECT 'related_tc', 50 UNION ALL
    SELECT 'related_row_id', 60 UNION ALL
    SELECT 'success', 70 UNION ALL
    SELECT 'ip_address', 80
) cols
WHERE tv.table_name = 'table_logs' AND tv.view_name = 'Audit log'
ON DUPLICATE KEY UPDATE is_visible = VALUES(is_visible), column_order = VALUES(column_order);
