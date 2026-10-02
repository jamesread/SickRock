INSERT OR IGNORE INTO table_configurations (name, title, db, "table", ordinal, icon)
VALUES ('table_dashboard_components', 'Dashboard components', 'main', 'table_dashboard_components', 8, 'LayoutIcon');

UPDATE table_configurations
SET title = 'Dashboard components', db = 'main', "table" = 'table_dashboard_components', ordinal = 8, icon = 'LayoutIcon'
WHERE name = 'table_dashboard_components';

INSERT OR IGNORE INTO table_views (table_name, view_name, is_default, view_type)
VALUES ('table_dashboard_components', 'Components', 1, 'table');

INSERT OR IGNORE INTO table_view_columns (view_id, column_name, is_visible, column_order, sort_order)
SELECT tv.id, cols.name, 1, cols.ord, CASE WHEN cols.name = 'ordinal' THEN 'asc' ELSE '' END
FROM table_views tv
JOIN (
    SELECT 'name' AS name, 10 AS ord UNION ALL
    SELECT 'dashboard', 20 UNION ALL
    SELECT 'query_type', 30 UNION ALL
    SELECT 'tc_id', 40 UNION ALL
    SELECT 'column_name', 50 UNION ALL
    SELECT 'formula', 60 UNION ALL
    SELECT 'ordinal', 70
) cols
WHERE tv.table_name = 'table_dashboard_components' AND tv.view_name = 'Components';
