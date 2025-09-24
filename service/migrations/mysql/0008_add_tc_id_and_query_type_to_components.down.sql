-- Drop tc_id FK and columns from table_dashboard_components (MySQL)

ALTER TABLE table_dashboard_components
  DROP FOREIGN KEY fk_dashboard_components_tc;

ALTER TABLE table_dashboard_components
  DROP COLUMN tc_id,
  DROP COLUMN query_type;
