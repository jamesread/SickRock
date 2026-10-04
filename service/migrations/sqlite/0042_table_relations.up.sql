CREATE TABLE IF NOT EXISTS table_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    constraint_name TEXT NOT NULL UNIQUE,
    source_table_key TEXT NOT NULL,
    source_column TEXT NOT NULL,
    referenced_table_key TEXT NOT NULL,
    referenced_column TEXT NOT NULL,
    on_delete_action TEXT NOT NULL DEFAULT 'NO ACTION',
    on_update_action TEXT NOT NULL DEFAULT 'NO ACTION',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_table_relations_unique
    ON table_relations (source_table_key, source_column, referenced_table_key, referenced_column);

CREATE INDEX IF NOT EXISTS idx_table_relations_source ON table_relations (source_table_key);
CREATE INDEX IF NOT EXISTS idx_table_relations_referenced ON table_relations (referenced_table_key);
