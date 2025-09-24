-- Add dashboard_component to table_dashboard_component_rules (SQLite)

ALTER TABLE table_dashboard_component_rules ADD COLUMN dashboard_component INTEGER;

-- Note: Enforcing FK in SQLite would require a table rebuild.
