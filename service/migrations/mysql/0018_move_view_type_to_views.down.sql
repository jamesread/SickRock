-- Revert: Move view type back from table_views to table_configurations

-- Add view column back to table_configurations
ALTER TABLE table_configurations
  ADD COLUMN IF NOT EXISTS view VARCHAR(255) NULL;

-- Migrate view_type back to table_configurations.view
-- Set view to 'calendar' where default view has view_type = 'calendar'
UPDATE table_configurations tc
INNER JOIN table_views tv ON tc.name = tv.table_name
SET tc.view = 'calendar'
WHERE tv.view_type = 'calendar' AND tv.is_default = TRUE;

-- Set default to 'table' for others
UPDATE table_configurations
SET view = 'table'
WHERE view IS NULL;

-- Remove view_type column from table_views
ALTER TABLE table_views
  DROP COLUMN IF EXISTS view_type;
