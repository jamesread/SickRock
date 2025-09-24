-- Dashboard component rules schema (MySQL)

CREATE TABLE IF NOT EXISTS table_dashboard_component_rules (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    component INT,
    ordinal INT,
    operation VARCHAR(255),
    operand VARCHAR(255)
);
