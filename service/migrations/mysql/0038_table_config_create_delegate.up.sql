ALTER TABLE table_configurations
  ADD COLUMN create_delegate VARCHAR(512) NULL;

UPDATE table_configurations
SET create_delegate = '/admin/table/create'
WHERE name = 'table_configurations';
