-- Initial schema for MySQL

CREATE TABLE IF NOT EXISTS table_configurations (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(191) NOT NULL UNIQUE,
    title VARCHAR(191),
    ordinal INT DEFAULT 0,
    db VARCHAR(191)
);

CREATE TABLE IF NOT EXISTS table_conditional_formatting_rules (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_name VARCHAR(191) NOT NULL,
    column_name VARCHAR(191) NOT NULL,
    condition_type VARCHAR(50) NOT NULL,
    condition_value TEXT,
    format_type VARCHAR(50) NOT NULL,
    format_value TEXT,
    priority INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP())
);

CREATE TABLE IF NOT EXISTS table_views (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_name VARCHAR(191) NOT NULL,
    view_name VARCHAR(191) NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP()),
    UNIQUE KEY unique_table_view (table_name, view_name)
);

CREATE TABLE IF NOT EXISTS table_view_columns (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    view_id INT NOT NULL,
    column_name VARCHAR(191) NOT NULL,
    is_visible BOOLEAN DEFAULT TRUE,
    column_order INT DEFAULT 0,
    column_width INT,
    sort_order VARCHAR(10),
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP()),
    FOREIGN KEY (view_id) REFERENCES table_views(id) ON DELETE CASCADE,
    UNIQUE KEY unique_view_column (view_id, column_name)
);

CREATE TABLE IF NOT EXISTS table_recently_viewed (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    table_id BIGINT NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP())
);

CREATE TABLE IF NOT EXISTS table_navigation (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    ordinal INT DEFAULT 99,
    table_configuration INT,
    FOREIGN KEY (table_configuration) REFERENCES table_configurations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS table_users (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) UNIQUE,
    password VARCHAR(1024),
    initial_route VARCHAR(255) DEFAULT '/'
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS table_sessions (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
    user_agent TEXT,
    ip_address VARCHAR(45),
    INDEX idx_session_id (session_id),
    INDEX idx_username (username),
    INDEX idx_expires_at (expires_at)
);

CREATE TABLE IF NOT EXISTS device_codes (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    code VARCHAR(4) NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    claimed_by VARCHAR(255),
    claimed_at DATETIME NULL,
    INDEX idx_code (code),
    INDEX idx_expires_at (expires_at),
    INDEX idx_claimed_by (claimed_by)
);
