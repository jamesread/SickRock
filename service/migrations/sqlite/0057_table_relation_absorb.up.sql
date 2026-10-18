-- Show the related rows of a relation inside the item (absorb_type 'first_row': the first
-- related row's fields appear as a subsection of the item's main section).
ALTER TABLE table_relations ADD COLUMN absorb_into_item INTEGER NOT NULL DEFAULT 0;
ALTER TABLE table_relations ADD COLUMN absorb_type TEXT;
