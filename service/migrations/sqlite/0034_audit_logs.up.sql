CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sr_created DATETIME NOT NULL DEFAULT (datetime('now')),
    event_type TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    actor_user_id INTEGER,
    actor_username TEXT NOT NULL DEFAULT '',
    related_user TEXT NOT NULL DEFAULT '',
    related_tc TEXT NOT NULL DEFAULT '',
    related_row_id TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    success INTEGER NOT NULL DEFAULT 1,
    details_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_sr_created ON audit_logs(sr_created DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_related_tc ON audit_logs(related_tc);

INSERT OR IGNORE INTO rbac_permissions (name, description) VALUES
('audit.view', 'View security and audit logs');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'superuser';

INSERT OR IGNORE INTO rbac_roles (name, description) VALUES
('auditor', 'View security and audit logs (system role)');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'audit.view'
WHERE r.name = 'auditor';

INSERT OR IGNORE INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g JOIN rbac_roles r ON r.name = 'auditor'
WHERE g.name = 'Administrators';

INSERT OR IGNORE INTO table_configurations (name, title, db, "table", ordinal, icon)
VALUES ('table_logs', 'Audit logs', 'main', 'audit_logs', 6, 'ShieldIcon');

INSERT OR IGNORE INTO table_views (table_name, view_name, is_default, view_type)
VALUES ('table_logs', 'Audit log', 1, 'table');

INSERT OR IGNORE INTO table_view_columns (view_id, column_name, is_visible, column_order, sort_order)
SELECT tv.id, cols.name, 1, cols.ord,
       CASE WHEN cols.name = 'sr_created' THEN 'desc' ELSE '' END
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
WHERE tv.table_name = 'table_logs' AND tv.view_name = 'Audit log';
