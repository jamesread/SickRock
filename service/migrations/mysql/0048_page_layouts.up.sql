CREATE TABLE IF NOT EXISTS page_layouts (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_configuration VARCHAR(255) NOT NULL,
    page_kind VARCHAR(32) NOT NULL DEFAULT 'item_view',
    UNIQUE KEY uk_page_layouts_tc_kind (table_configuration, page_kind)
);

CREATE TABLE IF NOT EXISTS page_layout_blocks (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    page_layout_id BIGINT NOT NULL,
    ordinal INT NOT NULL DEFAULT 0,
    block_type VARCHAR(32) NOT NULL,
    config_json TEXT NOT NULL,
    KEY idx_page_layout_blocks_layout (page_layout_id),
    CONSTRAINT fk_page_layout_blocks_layout FOREIGN KEY (page_layout_id) REFERENCES page_layouts(id) ON DELETE CASCADE
);
