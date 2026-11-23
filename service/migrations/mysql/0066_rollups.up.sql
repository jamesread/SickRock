-- Rollups: gather the detail rows owned by a set of source rows (e.g. the ingredients of the
-- recipes planned in a week), group and sum them, and replace a target table's rows with the result.
CREATE TABLE IF NOT EXISTS rollups (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    source_table VARCHAR(255) NOT NULL,
    source_link_column VARCHAR(255) NOT NULL DEFAULT '',
    detail_table VARCHAR(255) NOT NULL,
    detail_link_column VARCHAR(255) NOT NULL,
    group_columns_json TEXT NOT NULL DEFAULT '[]',
    sum_columns_json TEXT NOT NULL DEFAULT '[]',
    target_table VARCHAR(255) NOT NULL,
    column_map_json TEXT NOT NULL DEFAULT '{}',
    count_column VARCHAR(255) NOT NULL DEFAULT '',
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

INSERT INTO rbac_permissions (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'rollup.run', 'Run rollups, replacing the rows of their target table')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'rollup.run'
WHERE r.name IN ('superuser', 'member');
