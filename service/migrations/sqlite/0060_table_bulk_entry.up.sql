-- JSON object configuring a table's bulk entry page, e.g. {"scan_column":"barcode","quantity_column":"quantity","match_columns":["expirydate"],"fields":["title"]}.
ALTER TABLE table_configurations ADD COLUMN bulk_entry_json TEXT;
