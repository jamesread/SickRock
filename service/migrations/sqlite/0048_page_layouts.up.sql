CREATE TABLE IF NOT EXISTS page_layouts (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    table_configuration TEXT NOT NULL,
    page_kind TEXT NOT NULL DEFAULT 'item_view',
    UNIQUE (table_configuration, page_kind)
);

CREATE TABLE IF NOT EXISTS page_layout_blocks (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    page_layout_id INTEGER NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 0,
    block_type TEXT NOT NULL,
    config_json TEXT NOT NULL,
    FOREIGN KEY (page_layout_id) REFERENCES page_layouts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_page_layout_blocks_layout ON page_layout_blocks (page_layout_id);
