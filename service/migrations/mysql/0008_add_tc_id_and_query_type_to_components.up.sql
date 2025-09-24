-- Add tc_id and query_type to table_dashboard_components (MySQL)

ALTER TABLE table_dashboard_components
  ADD COLUMN tc_id INT NULL,
  ADD COLUMN query_type VARCHAR(255);

ALTER TABLE table_dashboard_components
  ADD CONSTRAINT fk_dashboard_components_tc
  FOREIGN KEY (tc_id) REFERENCES table_dashboard_components(id);
