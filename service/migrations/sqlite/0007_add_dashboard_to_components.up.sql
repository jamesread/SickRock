-- Add dashboard column to table_dashboard_components (SQLite)

ALTER TABLE table_dashboard_components ADD COLUMN dashboard INTEGER;

-- Note: To strictly enforce FK in SQLite, a table rebuild would be required.
