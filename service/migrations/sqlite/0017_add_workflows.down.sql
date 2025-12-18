-- Remove workflows table and workflow_id from table_navigation (SQLite)

-- Note: SQLite doesn't support DROP COLUMN directly in older versions.
-- This would require a table rebuild in production.

-- For now, we'll just drop the workflows table
DROP TABLE IF EXISTS table_workflows;
