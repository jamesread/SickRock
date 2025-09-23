-- Remove initial_route column from table_users (MySQL)

ALTER TABLE table_users
  DROP COLUMN initial_route;
