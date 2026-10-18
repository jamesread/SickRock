-- Full picocrank layout state (filters, group-by, page size, column priorities) for a view.
ALTER TABLE table_views ADD COLUMN IF NOT EXISTS layout_json TEXT NULL;

-- Each user's own default view per table (falls back to table_views.is_default).
CREATE TABLE IF NOT EXISTS table_view_user_defaults (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    table_name VARCHAR(191) NOT NULL,
    view_id INT NOT NULL,
    UNIQUE KEY uk_table_view_user_defaults (user_id, table_name),
    KEY idx_table_view_user_defaults_view (view_id)
);
