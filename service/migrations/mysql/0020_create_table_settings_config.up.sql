-- Create table configuration for table_settings (MySQL)
INSERT INTO table_configurations (name, title, `db`, `table`, ordinal)
VALUES ('table_settings', 'Settings', 'main', 'table_settings', 0)
ON DUPLICATE KEY UPDATE title = 'Settings', `db` = 'main', `table` = 'table_settings';
