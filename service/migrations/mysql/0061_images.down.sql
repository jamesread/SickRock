DROP TABLE IF EXISTS images;
DELETE FROM table_column_semantics WHERE semantic_type = 'image';
DELETE FROM table_configurations WHERE name = 'table_images';
