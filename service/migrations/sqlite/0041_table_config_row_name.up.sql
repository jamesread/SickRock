ALTER TABLE table_configurations ADD COLUMN row_name TEXT;

UPDATE table_configurations SET row_name = 'Setting' WHERE name = 'table_settings';
UPDATE table_configurations SET row_name = 'Table configuration' WHERE name = 'table_configurations';
UPDATE table_configurations SET row_name = 'Workflow' WHERE name = 'table_workflows';
UPDATE table_configurations SET row_name = 'Navigation item' WHERE name = 'table_navigation';
UPDATE table_configurations SET row_name = 'Dashboard' WHERE name = 'table_dashboards';
UPDATE table_configurations SET row_name = 'Dashboard component' WHERE name = 'table_dashboard_components';
UPDATE table_configurations SET row_name = 'Calendar export' WHERE name = 'table_read_only_exports';
UPDATE table_configurations SET row_name = 'Audit log entry' WHERE name = 'table_logs';
