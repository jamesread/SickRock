ALTER TABLE table_configurations
    ADD COLUMN primary_key_column VARCHAR(191) NULL,
    ADD COLUMN default_sort_column VARCHAR(191) NULL;
