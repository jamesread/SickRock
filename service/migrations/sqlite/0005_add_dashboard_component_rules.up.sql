-- Dashboard component rules schema (SQLite)

CREATE TABLE IF NOT EXISTS table_dashboard_component_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    component INTEGER,
    ordinal INTEGER,
    operation TEXT,
    operand TEXT
);
