-- Recreate legacy 'component' column on table_dashboard_component_rules (MySQL)

ALTER TABLE table_dashboard_component_rules
  ADD COLUMN component INT;
