-- Drop dashboard_component and its FK from rules (MySQL)

ALTER TABLE table_dashboard_component_rules
  DROP FOREIGN KEY fk_rules_dashboard_component;

ALTER TABLE table_dashboard_component_rules
  DROP COLUMN dashboard_component;
