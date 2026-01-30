-- Add name column to table_navigation (MySQL)

ALTER TABLE table_navigation
  ADD COLUMN name VARCHAR(255) NULL;
