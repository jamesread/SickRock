-- Add notification system tables (MySQL)

-- Table to store available notification events
CREATE TABLE IF NOT EXISTS notification_events (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    event_code VARCHAR(191) NOT NULL UNIQUE,
    event_name VARCHAR(255) NOT NULL,
    description TEXT,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Table to store user notification channel configurations
CREATE TABLE IF NOT EXISTS user_notification_channels (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user INT NOT NULL,
    channel_type VARCHAR(50) NOT NULL, -- 'email', 'telegram', 'webhook'
    channel_value TEXT NOT NULL, -- email address, telegram ID, or webhook URL
    channel_name VARCHAR(255), -- Optional name for webhooks (e.g., "Webhook A")
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user) REFERENCES table_users(id) ON DELETE CASCADE,
    UNIQUE KEY unique_user_channel (user, channel_type, channel_value(191))
) ENGINE=InnoDB;

-- Table to store user subscriptions to events via specific channels
CREATE TABLE IF NOT EXISTS user_notification_subscriptions (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user INT NOT NULL,
    event_id INT NOT NULL,
    channel_id INT NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user) REFERENCES table_users(id) ON DELETE CASCADE,
    FOREIGN KEY (event_id) REFERENCES notification_events(id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES user_notification_channels(id) ON DELETE CASCADE,
    UNIQUE KEY unique_user_event_channel (user, event_id, channel_id)
) ENGINE=InnoDB;

-- Insert default notification events
INSERT INTO notification_events (event_code, event_name, description) VALUES
    ('user.logged_in', 'User Logged In', 'Triggered when a user successfully logs in'),
    ('password.reset_reminder', 'Password Reset Reminder', 'Reminder to reset password')
ON DUPLICATE KEY UPDATE event_code = event_code;

CREATE INDEX idx_notification_channels_user ON user_notification_channels(user);
CREATE INDEX idx_notification_channels_type ON user_notification_channels(channel_type);
CREATE INDEX idx_notification_subscriptions_user ON user_notification_subscriptions(user);
CREATE INDEX idx_notification_subscriptions_event ON user_notification_subscriptions(event_id);
CREATE INDEX idx_notification_subscriptions_channel ON user_notification_subscriptions(channel_id);
