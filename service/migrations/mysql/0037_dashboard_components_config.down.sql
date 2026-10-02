DELETE FROM table_view_columns
WHERE view_id IN (SELECT id FROM table_views WHERE table_name = 'table_dashboard_components');
DELETE FROM table_views WHERE table_name = 'table_dashboard_components';
DELETE FROM table_configurations WHERE name = 'table_dashboard_components';
