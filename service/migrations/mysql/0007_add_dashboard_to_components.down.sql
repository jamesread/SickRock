-- Drop FK and column for dashboard in table_dashboard_components (MySQL)

ALTER TABLE table_dashboard_components
  DROP FOREIGN KEY fk_dashboard_components_dashboard;

ALTER TABLE table_dashboard_components
  DROP COLUMN dashboard;
