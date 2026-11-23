-- The target rows each rollup run wrote, so the next run replaces only those and keeps rows added
-- by hand (e.g. extra items on a shopping list).
CREATE TABLE IF NOT EXISTS rollup_rows (
    rollup_id BIGINT NOT NULL,
    target_table VARCHAR(255) NOT NULL,
    row_id VARCHAR(64) NOT NULL,
    PRIMARY KEY (rollup_id, target_table, row_id)
);

-- Rollups that already ran replaced every target row and recorded none, so their next run does that
-- once more (rows_tracked 0), and records its rows from then on.
ALTER TABLE rollups ADD COLUMN IF NOT EXISTS rows_tracked TINYINT(1) NOT NULL DEFAULT 1;
UPDATE rollups SET rows_tracked = 0;
