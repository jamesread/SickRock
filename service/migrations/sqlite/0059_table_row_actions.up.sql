-- JSON array of links shown on a row's item page, e.g. [{"label":"…","path":"/admin/table/create","query":{"navigationAppId":"{id}"}}].
ALTER TABLE table_configurations ADD COLUMN row_actions TEXT;
