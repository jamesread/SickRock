-- Rollups: gather the detail rows owned by a set of source rows (e.g. the ingredients of the
-- recipes planned in a week), group and sum them, and replace a target table's rows with the result.
CREATE TABLE IF NOT EXISTS rollups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    source_table TEXT NOT NULL,
    source_link_column TEXT NOT NULL DEFAULT '',
    detail_table TEXT NOT NULL,
    detail_link_column TEXT NOT NULL,
    group_columns_json TEXT NOT NULL DEFAULT '[]',
    sum_columns_json TEXT NOT NULL DEFAULT '[]',
    target_table TEXT NOT NULL,
    column_map_json TEXT NOT NULL DEFAULT '{}',
    count_column TEXT NOT NULL DEFAULT '',
    sr_created TEXT DEFAULT (datetime('now')),
    sr_updated TEXT DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO rbac_permissions (name, description) VALUES
('rollup.run', 'Run rollups, replacing the rows of their target table');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'rollup.run'
WHERE r.name IN ('superuser', 'member');
