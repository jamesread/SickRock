-- Dashboards schema (MySQL)

CREATE TABLE IF NOT EXISTS table_dashboards (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS table_dashboard_components (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255)
);
