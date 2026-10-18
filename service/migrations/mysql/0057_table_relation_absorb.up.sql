-- Show the related rows of a relation inside the item (absorb_type 'first_row': the first
-- related row's fields appear as a subsection of the item's main section).
ALTER TABLE table_relations
  ADD COLUMN IF NOT EXISTS absorb_into_item TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS absorb_type VARCHAR(32) NULL;
