-- Remove read_only from table_api_keys (SQLite)
-- SQLite does not support DROP COLUMN in older versions; use a rebuild if needed.
-- For simplicity we leave the column in place on down (or use a full table recreate).
CREATE TABLE IF NOT EXISTS table_api_keys_backup (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME NULL,
    expires_at DATETIME NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    FOREIGN KEY (user_id) REFERENCES table_users(id) ON DELETE CASCADE
);
INSERT INTO table_api_keys_backup (id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active)
SELECT id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active FROM table_api_keys;
DROP TABLE table_api_keys;
ALTER TABLE table_api_keys_backup RENAME TO table_api_keys;
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON table_api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON table_api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON table_api_keys(is_active);
