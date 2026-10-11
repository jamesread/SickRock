CREATE TABLE IF NOT EXISTS table_field_groups (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    table_configuration TEXT NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_table_field_groups_tc ON table_field_groups (table_configuration);

CREATE TABLE IF NOT EXISTS table_field_group_columns (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    field_group_id INTEGER NOT NULL,
    column_name TEXT NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 0,
    UNIQUE (field_group_id, column_name),
    FOREIGN KEY (field_group_id) REFERENCES table_field_groups (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_field_group_columns_group ON table_field_group_columns (field_group_id);
