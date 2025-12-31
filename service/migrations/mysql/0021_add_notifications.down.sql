-- Remove notification system tables (MySQL)

DROP INDEX IF EXISTS idx_notification_subscriptions_channel ON user_notification_subscriptions;
DROP INDEX IF EXISTS idx_notification_subscriptions_event ON user_notification_subscriptions;
DROP INDEX IF EXISTS idx_notification_subscriptions_user ON user_notification_subscriptions;
DROP INDEX IF EXISTS idx_notification_channels_type ON user_notification_channels;
DROP INDEX IF EXISTS idx_notification_channels_user ON user_notification_channels;

DROP TABLE IF EXISTS user_notification_subscriptions;
DROP TABLE IF EXISTS user_notification_channels;
DROP TABLE IF EXISTS notification_events;
