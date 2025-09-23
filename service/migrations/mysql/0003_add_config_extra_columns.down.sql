-- Remove configuration extra columns

ALTER TABLE table_configurations
  DROP COLUMN create_button_text,
  DROP COLUMN icon,
  DROP COLUMN view,
  DROP COLUMN `table`;
