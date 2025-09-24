-- Update foreign key fk_dashboard_components_tc to reference table_configurations(id) (MySQL)

ALTER TABLE table_dashboard_components
  DROP FOREIGN KEY fk_dashboard_components_tc;

ALTER TABLE table_dashboard_components
  ADD CONSTRAINT fk_dashboard_components_tc
  FOREIGN KEY (tc_id) REFERENCES table_configurations(id);
