-- Add columns used by configuration features (SQLite)

ALTER TABLE table_configurations
  ADD COLUMN create_button_text TEXT;

ALTER TABLE table_configurations
  ADD COLUMN icon TEXT;

ALTER TABLE table_configurations
  ADD COLUMN view TEXT;

ALTER TABLE table_configurations
  ADD COLUMN `table` TEXT;
