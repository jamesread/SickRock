-- Add dashboard_id to table_navigation and reference table_dashboards(id) (MySQL)

ALTER TABLE table_navigation
  ADD COLUMN dashboard_id INT NULL;

ALTER TABLE table_navigation
  ADD CONSTRAINT fk_table_navigation_dashboard
  FOREIGN KEY (dashboard_id) REFERENCES table_dashboards(id);
