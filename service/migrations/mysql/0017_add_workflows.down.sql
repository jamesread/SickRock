-- Remove workflows table and workflow_id from table_navigation (MySQL)

ALTER TABLE table_navigation
  DROP FOREIGN KEY fk_table_navigation_workflow;

ALTER TABLE table_navigation
  DROP COLUMN workflow_id;

DROP TABLE IF EXISTS table_workflows;
