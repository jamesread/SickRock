-- Add dashboard column to table_dashboard_components with FK to table_dashboards(id) (MySQL)

ALTER TABLE table_dashboard_components
  ADD COLUMN dashboard INT NULL;

ALTER TABLE table_dashboard_components
  ADD CONSTRAINT fk_dashboard_components_dashboard
  FOREIGN KEY (dashboard) REFERENCES table_dashboards(id);
