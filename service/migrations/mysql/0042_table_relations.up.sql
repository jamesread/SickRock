CREATE TABLE IF NOT EXISTS table_relations (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    constraint_name VARCHAR(255) NOT NULL,
    source_table_key VARCHAR(255) NOT NULL,
    source_column VARCHAR(255) NOT NULL,
    referenced_table_key VARCHAR(255) NOT NULL,
    referenced_column VARCHAR(255) NOT NULL,
    on_delete_action VARCHAR(32) NOT NULL DEFAULT 'NO ACTION',
    on_update_action VARCHAR(32) NOT NULL DEFAULT 'NO ACTION',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_table_relations_constraint_name (constraint_name),
    UNIQUE KEY uk_table_relations_unique (source_table_key, source_column, referenced_table_key, referenced_column),
    KEY idx_table_relations_source (source_table_key),
    KEY idx_table_relations_referenced (referenced_table_key)
);
