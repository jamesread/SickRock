-- Add dashboard_id to table_navigation (SQLite)

ALTER TABLE table_navigation ADD COLUMN dashboard_id INTEGER;

-- Note: SQLite supports foreign keys but ALTER TABLE ... ADD CONSTRAINT is limited.
-- If strict enforcement is required, a full table rebuild would be needed.
