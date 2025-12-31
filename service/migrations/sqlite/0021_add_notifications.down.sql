-- Remove notification system tables (SQLite)

DROP INDEX IF EXISTS idx_notification_subscriptions_channel;
DROP INDEX IF EXISTS idx_notification_subscriptions_event;
DROP INDEX IF EXISTS idx_notification_subscriptions_user;
DROP INDEX IF EXISTS idx_notification_channels_type;
DROP INDEX IF EXISTS idx_notification_channels_user;

DROP TABLE IF EXISTS user_notification_subscriptions;
DROP TABLE IF EXISTS user_notification_channels;
DROP TABLE IF EXISTS notification_events;
