DELETE FROM rbac_resource_grants WHERE resource_type = 'workflow';

INSERT OR IGNORE INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'workflow', CAST(tw.id AS TEXT), 1, 0, 0, 0, 1
FROM user_groups g CROSS JOIN table_workflows tw
WHERE g.name = 'Everyone';
