DELETE FROM rbac_role_permissions WHERE permission_id IN (SELECT id FROM rbac_permissions WHERE name = 'rollup.run');
DELETE FROM rbac_permissions WHERE name = 'rollup.run';
DELETE FROM table_configurations WHERE name = 'table_rollups';
DROP TABLE IF EXISTS rollups;
