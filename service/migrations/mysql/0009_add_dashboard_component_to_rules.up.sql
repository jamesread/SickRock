-- Add dashboard_component to table_dashboard_component_rules (MySQL)

ALTER TABLE table_dashboard_component_rules
  ADD COLUMN dashboard_component INT NULL;

ALTER TABLE table_dashboard_component_rules
  ADD CONSTRAINT fk_rules_dashboard_component
  FOREIGN KEY (dashboard_component) REFERENCES table_dashboard_components(id);
