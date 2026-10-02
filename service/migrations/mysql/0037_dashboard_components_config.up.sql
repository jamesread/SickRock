INSERT INTO table_configurations (name, title, `db`, `table`, ordinal, icon)
VALUES ('table_dashboard_components', 'Dashboard components', 'main', 'table_dashboard_components', 8, 'LayoutIcon')
ON DUPLICATE KEY UPDATE title = VALUES(title), `table` = VALUES(`table`), ordinal = VALUES(ordinal), icon = VALUES(icon);

INSERT INTO table_views (table_name, view_name, is_default, view_type)
VALUES ('table_dashboard_components', 'Components', 1, 'table')
ON DUPLICATE KEY UPDATE is_default = VALUES(is_default);

INSERT INTO table_view_columns (view_id, column_name, is_visible, column_order, sort_order)
SELECT tv.id, cols.name, 1, cols.ord, IF(cols.name = 'ordinal', 'asc', '')
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
WHERE tv.table_name = 'table_dashboard_components' AND tv.view_name = 'Components'
ON DUPLICATE KEY UPDATE is_visible = VALUES(is_visible), column_order = VALUES(column_order);
