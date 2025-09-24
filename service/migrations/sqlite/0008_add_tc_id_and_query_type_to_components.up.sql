-- Add tc_id and query_type to table_dashboard_components (SQLite)

ALTER TABLE table_dashboard_components ADD COLUMN tc_id INTEGER;
ALTER TABLE table_dashboard_components ADD COLUMN query_type TEXT;

-- Note: To enforce FK, table rebuild would be required in SQLite.
