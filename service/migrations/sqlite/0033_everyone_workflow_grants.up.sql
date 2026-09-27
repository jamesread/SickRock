-- Everyone inherited every workflow; that exposed hub links to users with no usable member tables.
DELETE FROM rbac_resource_grants
WHERE resource_type = 'workflow'
  AND user_group_id IN (SELECT id FROM user_groups WHERE name = 'Everyone');

-- Keep full workflow access for Administrators.
INSERT OR IGNORE INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
SELECT g.id, 'workflow', CAST(tw.id AS TEXT), 1, 0, 0, 0, 1
FROM user_groups g
CROSS JOIN table_workflows tw
WHERE g.name = 'Administrators';
