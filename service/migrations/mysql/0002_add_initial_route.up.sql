-- Add initial_route column to table_users (MySQL)

ALTER TABLE table_users
  ADD COLUMN IF NOT EXISTS initial_route VARCHAR(255) DEFAULT '/';
