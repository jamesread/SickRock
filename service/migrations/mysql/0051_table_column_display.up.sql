CREATE TABLE IF NOT EXISTS table_column_display (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_configuration VARCHAR(255) NOT NULL,
    column_name VARCHAR(255) NOT NULL,
    display_label VARCHAR(255) NULL,
    value_prefix VARCHAR(512) NULL,
    value_suffix VARCHAR(512) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_table_column_display (table_configuration, column_name),
    KEY idx_table_column_display_tc (table_configuration)
);
