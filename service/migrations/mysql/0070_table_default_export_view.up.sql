-- The view the Export page starts on for a table (table structure page → Export): a table_views id,
-- -1 for All Columns, or NULL for the table's default view.
ALTER TABLE table_configurations ADD COLUMN IF NOT EXISTS default_export_view_id INT NULL;
