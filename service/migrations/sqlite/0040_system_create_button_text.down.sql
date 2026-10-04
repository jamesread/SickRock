UPDATE table_configurations SET create_button_text = NULL
WHERE name IN (
  'table_settings',
  'table_configurations',
  'table_workflows',
  'table_navigation',
  'table_dashboards',
  'table_dashboard_components',
  'table_read_only_exports',
  'table_logs'
);
