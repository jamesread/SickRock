-- Add formula to table_dashboard_components (MySQL)

ALTER TABLE table_dashboard_components
  ADD COLUMN formula VARCHAR(1024);
