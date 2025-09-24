-- Dashboards schema (SQLite)

CREATE TABLE IF NOT EXISTS table_dashboards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT
);

CREATE TABLE IF NOT EXISTS table_dashboard_components (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT
);
