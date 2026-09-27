-- Workflow hub membership (separate from table_navigation.workflow_id link-to-workflow)

CREATE TABLE IF NOT EXISTS workflow_navigation_members (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    workflow_id INT NOT NULL,
    navigation_item_id INT NOT NULL,
    ordinal INT NOT NULL DEFAULT 99,
    UNIQUE KEY uk_workflow_nav_member (workflow_id, navigation_item_id),
    CONSTRAINT fk_wnm_workflow FOREIGN KEY (workflow_id) REFERENCES table_workflows(id) ON DELETE CASCADE,
    CONSTRAINT fk_wnm_navigation FOREIGN KEY (navigation_item_id) REFERENCES table_navigation(id) ON DELETE CASCADE
);

-- Move mistaken membership: rows that point at a table/dashboard but had workflow_id set
INSERT INTO workflow_navigation_members (workflow_id, navigation_item_id, ordinal)
SELECT tn.workflow_id, tn.id, COALESCE(tn.ordinal, 99)
FROM table_navigation tn
WHERE tn.workflow_id IS NOT NULL
  AND (
    (tn.table_configuration IS NOT NULL AND tn.table_configuration <> 0)
    OR (tn.dashboard_id IS NOT NULL AND tn.dashboard_id <> 0)
  );

UPDATE table_navigation tn
SET tn.workflow_id = NULL
WHERE tn.workflow_id IS NOT NULL
  AND (
    (tn.table_configuration IS NOT NULL AND tn.table_configuration <> 0)
    OR (tn.dashboard_id IS NOT NULL AND tn.dashboard_id <> 0)
  );
