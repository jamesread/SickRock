DELETE FROM rbac_role_permissions WHERE permission_id IN (SELECT id FROM rbac_permissions WHERE name = 'exports.manage');
DELETE FROM rbac_permissions WHERE name = 'exports.manage';
DROP TABLE IF EXISTS read_only_calendar_exports;
