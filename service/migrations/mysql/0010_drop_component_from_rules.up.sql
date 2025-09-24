-- Drop legacy 'component' column from table_dashboard_component_rules (MySQL)

ALTER TABLE table_dashboard_component_rules
  DROP COLUMN component;
