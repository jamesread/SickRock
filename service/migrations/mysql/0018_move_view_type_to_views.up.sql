-- Move view type from table_configurations to table_views
-- Add view_type column to table_views with default 'table'
ALTER TABLE table_views
  ADD COLUMN IF NOT EXISTS view_type VARCHAR(50) NOT NULL DEFAULT 'table';

-- Migrate existing calendar views from table_configurations to table_views
-- Set view_type to 'calendar' for default views where table_configurations.view = 'calendar'
UPDATE table_views tv
INNER JOIN table_configurations tc ON tv.table_name = tc.name
SET tv.view_type = 'calendar'
WHERE tc.view = 'calendar' AND tv.is_default = TRUE;

-- Remove view column from table_configurations
ALTER TABLE table_configurations
  DROP COLUMN IF EXISTS view;
