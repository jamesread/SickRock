-- The target rows each rollup run wrote, so the next run replaces only those and keeps rows added
-- by hand (e.g. extra items on a shopping list).
CREATE TABLE IF NOT EXISTS rollup_rows (
    rollup_id INTEGER NOT NULL,
    target_table TEXT NOT NULL,
    row_id TEXT NOT NULL,
    PRIMARY KEY (rollup_id, target_table, row_id)
);

-- Rollups that already ran replaced every target row and recorded none, so their next run does that
-- once more (rows_tracked 0), and records its rows from then on.
ALTER TABLE rollups ADD COLUMN rows_tracked INTEGER NOT NULL DEFAULT 1;
UPDATE rollups SET rows_tracked = 0;
