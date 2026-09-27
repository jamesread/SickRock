DELETE FROM rbac_table_group_roles WHERE table_key = 'table_logs';
DELETE FROM table_view_columns WHERE view_id IN (SELECT id FROM table_views WHERE table_name = 'table_logs');
DELETE FROM table_views WHERE table_name = 'table_logs';
DELETE FROM table_configurations WHERE name = 'table_logs';
DELETE FROM rbac_role_permissions WHERE permission_id IN (SELECT id FROM rbac_permissions WHERE name = 'audit.view');
DELETE FROM rbac_permissions WHERE name = 'audit.view';
DROP TABLE IF EXISTS audit_logs;
