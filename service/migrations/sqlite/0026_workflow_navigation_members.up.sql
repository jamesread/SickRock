CREATE TABLE IF NOT EXISTS workflow_navigation_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id INTEGER NOT NULL,
    navigation_item_id INTEGER NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 99,
    UNIQUE (workflow_id, navigation_item_id),
    FOREIGN KEY (workflow_id) REFERENCES table_workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (navigation_item_id) REFERENCES table_navigation(id) ON DELETE CASCADE
);

INSERT INTO workflow_navigation_members (workflow_id, navigation_item_id, ordinal)
SELECT tn.workflow_id, tn.id, COALESCE(tn.ordinal, 99)
FROM table_navigation tn
WHERE tn.workflow_id IS NOT NULL
  AND (
    (tn.table_configuration IS NOT NULL AND tn.table_configuration <> 0)
    OR (tn.dashboard_id IS NOT NULL AND tn.dashboard_id <> 0)
  );

UPDATE table_navigation
SET workflow_id = NULL
WHERE workflow_id IS NOT NULL
  AND (
    (table_configuration IS NOT NULL AND table_configuration <> 0)
    OR (dashboard_id IS NOT NULL AND dashboard_id <> 0)
  );
