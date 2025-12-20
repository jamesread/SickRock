-- Create table_settings table (SQLite)
CREATE TABLE IF NOT EXISTS table_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    setting_key TEXT NOT NULL UNIQUE,
    string_val TEXT,
    bool_val INTEGER,
    title TEXT,
    sr_created TEXT DEFAULT (datetime('now')),
    sr_updated TEXT DEFAULT (datetime('now'))
);

-- Insert initial appTitle setting
INSERT OR IGNORE INTO table_settings (setting_key, string_val, title)
VALUES ('appTitle', 'SickRock', 'Application Title');
