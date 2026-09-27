CREATE TABLE IF NOT EXISTS read_only_calendar_exports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    table_configuration TEXT NOT NULL,
    table_view_id INTEGER,
    where_json TEXT NOT NULL DEFAULT '{}',
    weekends_only INTEGER NOT NULL DEFAULT 0,
    display_mode TEXT NOT NULL DEFAULT 'full',
    enabled INTEGER NOT NULL DEFAULT 1,
    allowed_group_ids TEXT NOT NULL DEFAULT '[]',
    sr_created TEXT DEFAULT (datetime('now')),
    sr_updated TEXT DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO rbac_permissions (name, description) VALUES
('exports.manage', 'Create and edit read-only calendar export definitions');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'exports.manage'
WHERE r.name = 'superuser';
