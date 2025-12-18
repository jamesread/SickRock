-- Add workflows table and workflow_id to table_navigation (MySQL)

CREATE TABLE IF NOT EXISTS table_workflows (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    ordinal INT DEFAULT 99,
    icon VARCHAR(255)
);

ALTER TABLE table_navigation
  ADD COLUMN workflow_id INT NULL;

ALTER TABLE table_navigation
  ADD CONSTRAINT fk_table_navigation_workflow
  FOREIGN KEY (workflow_id) REFERENCES table_workflows(id) ON DELETE SET NULL;
