-- Full picocrank layout state (filters, group-by, page size, column priorities) for a view.
ALTER TABLE table_views ADD COLUMN layout_json TEXT;

-- Each user's own default view per table (falls back to table_views.is_default).
CREATE TABLE IF NOT EXISTS table_view_user_defaults (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    table_name TEXT NOT NULL,
    view_id INTEGER NOT NULL,
    UNIQUE (user_id, table_name)
);

CREATE INDEX IF NOT EXISTS idx_table_view_user_defaults_view ON table_view_user_defaults (view_id);
