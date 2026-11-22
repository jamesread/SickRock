-- User-to-user notifications: an event recipients subscribe to, and the permission to send it.

INSERT OR IGNORE INTO notification_events (event_code, event_name, description) VALUES
    ('user.notified', 'Notified by another user', 'Another user sent you a notification, e.g. that a weekly plan is ready');

INSERT OR IGNORE INTO rbac_permissions (name, description) VALUES
('notifications.send', 'Send notifications to users who share a group');

INSERT OR IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'notifications.send'
WHERE r.name IN ('superuser', 'member');

-- Notification channels and subscriptions still referenced the legacy table_users, so users
-- created since armature-iam (only in user_accounts) could not add a channel. Rebuild both
-- tables with their user foreign keys on user_accounts, dropping rows for users that are gone.
CREATE TABLE user_notification_channels_new (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user INTEGER NOT NULL,
    channel_type TEXT NOT NULL,
    channel_value TEXT NOT NULL,
    channel_name TEXT,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user) REFERENCES user_accounts(id) ON DELETE CASCADE,
    UNIQUE(user, channel_type, channel_value)
);
INSERT INTO user_notification_channels_new (id, user, channel_type, channel_value, channel_name, is_active, sr_created, sr_updated)
SELECT id, user, channel_type, channel_value, channel_name, is_active, sr_created, sr_updated
FROM user_notification_channels WHERE user IN (SELECT id FROM user_accounts);

CREATE TABLE user_notification_subscriptions_new (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user INTEGER NOT NULL,
    event_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user) REFERENCES user_accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (event_id) REFERENCES notification_events(id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES user_notification_channels_new(id) ON DELETE CASCADE,
    UNIQUE(user, event_id, channel_id)
);
INSERT INTO user_notification_subscriptions_new (id, user, event_id, channel_id, sr_created)
SELECT id, user, event_id, channel_id, sr_created
FROM user_notification_subscriptions
WHERE user IN (SELECT id FROM user_accounts)
  AND channel_id IN (SELECT id FROM user_notification_channels_new);

DROP TABLE user_notification_subscriptions;
DROP TABLE user_notification_channels;
-- Renaming also updates the subscriptions' channel_id reference.
ALTER TABLE user_notification_channels_new RENAME TO user_notification_channels;
ALTER TABLE user_notification_subscriptions_new RENAME TO user_notification_subscriptions;

CREATE INDEX IF NOT EXISTS idx_notification_channels_user ON user_notification_channels(user);
CREATE INDEX IF NOT EXISTS idx_notification_channels_type ON user_notification_channels(channel_type);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_user ON user_notification_subscriptions(user);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_event ON user_notification_subscriptions(event_id);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_channel ON user_notification_subscriptions(channel_id);
