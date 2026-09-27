UPDATE rbac_roles SET name = 'viewer', description = 'Read-only access to granted tables and workflows (system role)'
WHERE name = 'reader';

INSERT OR IGNORE INTO rbac_permissions (name, description) VALUES
('workflow.view', 'View workflow hubs and their navigation members'),
('workflow.start', 'Use workflow actions (mutating workflow membership and related writes)'),
('table.view', 'View table data and structure for granted tables'),
('table.insert', 'Insert rows in granted tables'),
('table.edit', 'Edit rows in granted tables'),
('table.delete', 'Delete rows in granted tables'),
('dashboard.view', 'View granted dashboards');

CREATE TABLE IF NOT EXISTS rbac_resource_grants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_group_id INTEGER NOT NULL,
    resource_type TEXT NOT NULL,
    resource_key TEXT NOT NULL,
    can_view INTEGER NOT NULL DEFAULT 0,
    can_insert INTEGER NOT NULL DEFAULT 0,
    can_edit INTEGER NOT NULL DEFAULT 0,
    can_delete INTEGER NOT NULL DEFAULT 0,
    can_start INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_group_id, resource_type, resource_key)
);

CREATE INDEX IF NOT EXISTS idx_rbac_resource_grants_group ON rbac_resource_grants(user_group_id);
CREATE INDEX IF NOT EXISTS idx_rbac_resource_grants_lookup ON rbac_resource_grants(resource_type, resource_key);

-- viewer: read-oriented global caps (resource grants still required)
INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name IN ('workflow.view', 'table.view', 'dashboard.view')
WHERE r.name = 'viewer';

-- member: full table/workflow capabilities (grants still scope which resources)
INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name IN (
  'workflow.view', 'workflow.start', 'table.view', 'table.insert', 'table.edit', 'table.delete', 'dashboard.view'
) WHERE r.name = 'member';

-- Seed Everyone: full access to all current tables, dashboards, and workflows
INSERT OR IGNORE INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'table', tc.name, 1, 1, 1, 1, 0
FROM user_groups g CROSS JOIN table_configurations tc
WHERE g.name = 'Everyone';

INSERT OR IGNORE INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'dashboard', td.name, 1, 0, 0, 0, 0
FROM user_groups g CROSS JOIN table_dashboards td
WHERE g.name = 'Everyone';

INSERT OR IGNORE INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'workflow', CAST(tw.id AS TEXT), 1, 0, 0, 0, 1
FROM user_groups g CROSS JOIN table_workflows tw
WHERE g.name = 'Everyone';
