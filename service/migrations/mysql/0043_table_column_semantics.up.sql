CREATE TABLE IF NOT EXISTS table_column_semantics (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_configuration VARCHAR(255) NOT NULL,
    column_name VARCHAR(255) NOT NULL,
    semantic_type VARCHAR(64) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_table_column_semantics (table_configuration, column_name),
    KEY idx_table_column_semantics_tc (table_configuration)
);
