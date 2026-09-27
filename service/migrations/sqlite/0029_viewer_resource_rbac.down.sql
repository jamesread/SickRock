DELETE FROM rbac_resource_grants;
DROP TABLE IF EXISTS rbac_resource_grants;

DELETE FROM rbac_role_permissions
WHERE permission_id IN (SELECT id FROM rbac_permissions WHERE name IN (
  'workflow.view', 'workflow.start', 'table.view', 'table.insert', 'table.edit', 'table.delete', 'dashboard.view'
));

DELETE FROM rbac_permissions WHERE name IN (
  'workflow.view', 'workflow.start', 'table.view', 'table.insert', 'table.edit', 'table.delete', 'dashboard.view'
);

UPDATE rbac_roles SET name = 'reader', description = 'Read-only access to shared resources (system role)'
WHERE name = 'viewer';
