-- Quick add for a relation's related rows panel (e.g. a recipe's ingredients): which columns the
-- inline entry row shows, and whether typing an unknown name in a relation column creates that row.
ALTER TABLE table_relations ADD COLUMN IF NOT EXISTS quick_add_json TEXT NULL;
