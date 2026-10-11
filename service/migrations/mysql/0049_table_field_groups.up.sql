CREATE TABLE IF NOT EXISTS table_field_groups (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_configuration VARCHAR(255) NOT NULL,
    ordinal INT NOT NULL DEFAULT 0,
    title VARCHAR(255) NOT NULL,
    KEY idx_table_field_groups_tc (table_configuration)
);

CREATE TABLE IF NOT EXISTS table_field_group_columns (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    field_group_id BIGINT NOT NULL,
    column_name VARCHAR(255) NOT NULL,
    ordinal INT NOT NULL DEFAULT 0,
    UNIQUE KEY uk_field_group_column (field_group_id, column_name),
    KEY idx_field_group_columns_group (field_group_id),
    CONSTRAINT fk_field_group_columns_group FOREIGN KEY (field_group_id) REFERENCES table_field_groups (id) ON DELETE CASCADE
);
