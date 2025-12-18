-- Add workflows table and workflow_id to table_navigation (SQLite)

CREATE TABLE IF NOT EXISTS table_workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    ordinal INTEGER DEFAULT 99,
    icon TEXT
);

ALTER TABLE table_navigation ADD COLUMN workflow_id INTEGER;

-- Note: SQLite supports foreign keys but ALTER TABLE ... ADD CONSTRAINT is limited.
-- If strict enforcement is required, a full table rebuild would be needed.
