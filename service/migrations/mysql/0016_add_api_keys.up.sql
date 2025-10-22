-- Add table_api_keys table (MySQL)

CREATE TABLE IF NOT EXISTS table_api_keys (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME NULL,
    expires_at DATETIME NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    FOREIGN KEY (user_id) REFERENCES table_users(id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_user_id ON table_api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON table_api_keys(key_hash);
CREATE INDEX idx_api_keys_active ON table_api_keys(is_active);
