-- Initial schema for SQLite

CREATE TABLE IF NOT EXISTS table_configurations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    title TEXT,
    ordinal INTEGER DEFAULT 0,
    db TEXT
);

CREATE TABLE IF NOT EXISTS table_conditional_formatting_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    condition_type TEXT NOT NULL,
    condition_value TEXT,
    format_type TEXT NOT NULL,
    format_value TEXT,
    priority INTEGER DEFAULT 0,
    is_active INTEGER DEFAULT 1,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now'))
);

CREATE TABLE IF NOT EXISTS table_views (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    view_name TEXT NOT NULL,
    is_default INTEGER DEFAULT 0,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(table_name, view_name)
);

CREATE TABLE IF NOT EXISTS table_view_columns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    view_id INTEGER NOT NULL,
    column_name TEXT NOT NULL,
    is_visible INTEGER DEFAULT 1,
    column_order INTEGER DEFAULT 0,
    column_width INTEGER,
    sort_order TEXT,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (view_id) REFERENCES table_views(id) ON DELETE CASCADE,
    UNIQUE(view_id, column_name)
);

CREATE TABLE IF NOT EXISTS table_recently_viewed (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    table_id INTEGER NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now'))
);

CREATE TABLE IF NOT EXISTS table_navigation (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    ordinal INTEGER DEFAULT 99,
    table_configuration INTEGER,
    FOREIGN KEY (table_configuration) REFERENCES table_configurations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS table_users (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE,
    password TEXT,
    initial_route TEXT DEFAULT '/'
);

CREATE TABLE IF NOT EXISTS table_sessions (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
    user_agent TEXT,
    ip_address TEXT,
    FOREIGN KEY (username) REFERENCES table_users(username) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_id ON table_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_username ON table_sessions(username);
CREATE INDEX IF NOT EXISTS idx_expires_at ON table_sessions(expires_at);

CREATE TABLE IF NOT EXISTS device_codes (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    claimed_by TEXT,
    claimed_at DATETIME NULL
);

CREATE INDEX IF NOT EXISTS idx_device_code ON device_codes(code);
CREATE INDEX IF NOT EXISTS idx_device_expires_at ON device_codes(expires_at);
CREATE INDEX IF NOT EXISTS idx_device_claimed_by ON device_codes(claimed_by);
