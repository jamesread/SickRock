CREATE TABLE IF NOT EXISTS table_column_semantics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_configuration TEXT NOT NULL,
    column_name TEXT NOT NULL,
    semantic_type TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (table_configuration, column_name)
);

CREATE INDEX IF NOT EXISTS idx_table_column_semantics_tc
    ON table_column_semantics (table_configuration);
