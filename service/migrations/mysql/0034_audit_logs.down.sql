DELETE FROM rbac_table_group_roles WHERE table_key = 'table_logs';
DELETE tvc FROM table_view_columns tvc
INNER JOIN table_views tv ON tv.id = tvc.view_id
WHERE tv.table_name = 'table_logs';
DELETE FROM table_views WHERE table_name = 'table_logs';
DELETE FROM table_configurations WHERE name = 'table_logs';
DELETE rp FROM rbac_role_permissions rp INNER JOIN rbac_permissions p ON p.id = rp.permission_id WHERE p.name = 'audit.view';
DELETE FROM rbac_permissions WHERE name = 'audit.view';
DROP TABLE IF EXISTS audit_logs;
