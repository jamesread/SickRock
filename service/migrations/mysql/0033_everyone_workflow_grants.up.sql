DELETE g FROM rbac_resource_grants g
INNER JOIN user_groups ug ON ug.id = g.user_group_id
WHERE g.resource_type = 'workflow' AND ug.name = 'Everyone';

INSERT INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'workflow', CAST(tw.id AS CHAR), 1, 0, 0, 0, 1
FROM user_groups g
CROSS JOIN table_workflows tw
WHERE g.name = 'Administrators'
ON DUPLICATE KEY UPDATE can_view = 1, can_start = 1;
