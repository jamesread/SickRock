-- Remove dashboard_id and its foreign key (MySQL)

ALTER TABLE table_navigation
  DROP FOREIGN KEY fk_table_navigation_dashboard;

ALTER TABLE table_navigation
  DROP COLUMN dashboard_id;
