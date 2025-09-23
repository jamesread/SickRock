-- Add columns used by configuration features

ALTER TABLE table_configurations
  ADD COLUMN IF NOT EXISTS create_button_text VARCHAR(255) NULL,
  ADD COLUMN IF NOT EXISTS icon VARCHAR(255) NULL,
  ADD COLUMN IF NOT EXISTS view VARCHAR(255) NULL,
  ADD COLUMN IF NOT EXISTS `table` VARCHAR(255) NULL;
