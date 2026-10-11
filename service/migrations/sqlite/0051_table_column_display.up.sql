CREATE TABLE IF NOT EXISTS table_column_display (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_configuration TEXT NOT NULL,
    column_name TEXT NOT NULL,
    display_label TEXT,
    value_prefix TEXT,
    value_suffix TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (table_configuration, column_name)
);

CREATE INDEX IF NOT EXISTS idx_table_column_display_tc
    ON table_column_display (table_configuration);
