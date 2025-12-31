-- Add notification system tables (SQLite)

-- Table to store available notification events
CREATE TABLE IF NOT EXISTS notification_events (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    event_code TEXT NOT NULL UNIQUE,
    event_name TEXT NOT NULL,
    description TEXT,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Table to store user notification channel configurations
CREATE TABLE IF NOT EXISTS user_notification_channels (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user INTEGER NOT NULL,
    channel_type TEXT NOT NULL, -- 'email', 'telegram', 'webhook'
    channel_value TEXT NOT NULL, -- email address, telegram ID, or webhook URL
    channel_name TEXT, -- Optional name for webhooks (e.g., "Webhook A")
    is_active BOOLEAN NOT NULL DEFAULT 1,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user) REFERENCES table_users(id) ON DELETE CASCADE,
    UNIQUE(user, channel_type, channel_value)
);

-- Table to store user subscriptions to events via specific channels
CREATE TABLE IF NOT EXISTS user_notification_subscriptions (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user INTEGER NOT NULL,
    event_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user) REFERENCES table_users(id) ON DELETE CASCADE,
    FOREIGN KEY (event_id) REFERENCES notification_events(id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES user_notification_channels(id) ON DELETE CASCADE,
    UNIQUE(user, event_id, channel_id)
);

-- Insert default notification events
INSERT INTO notification_events (event_code, event_name, description) VALUES
    ('user.logged_in', 'User Logged In', 'Triggered when a user successfully logs in'),
    ('password.reset_reminder', 'Password Reset Reminder', 'Reminder to reset password')
ON CONFLICT(event_code) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_notification_channels_user ON user_notification_channels(user);
CREATE INDEX IF NOT EXISTS idx_notification_channels_type ON user_notification_channels(channel_type);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_user ON user_notification_subscriptions(user);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_event ON user_notification_subscriptions(event_id);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_channel ON user_notification_subscriptions(channel_id);
