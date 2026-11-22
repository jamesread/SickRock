-- User-to-user notifications: an event recipients subscribe to, and the permission to send it.

INSERT INTO notification_events (event_code, event_name, description) VALUES
    ('user.notified', 'Notified by another user', 'Another user sent you a notification, e.g. that a weekly plan is ready')
ON DUPLICATE KEY UPDATE event_name = VALUES(event_name), description = VALUES(description);

INSERT INTO rbac_permissions (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'notifications.send', 'Send notifications to users who share a group')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT IGNORE INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'notifications.send'
WHERE r.name IN ('superuser', 'member');

-- Notification channels and subscriptions still referenced the legacy table_users, so users
-- created since armature-iam (only in user_accounts) could not add a channel. Move their user
-- foreign keys to user_accounts, dropping rows for users that are gone. Older databases may
-- not have the table_users constraints, so they are looked up by what they reference.
DELETE FROM user_notification_subscriptions WHERE user NOT IN (SELECT id FROM user_accounts);
DELETE FROM user_notification_channels WHERE user NOT IN (SELECT id FROM user_accounts);

SET @fk := (SELECT CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_notification_subscriptions'
    AND COLUMN_NAME = 'user' AND REFERENCED_TABLE_NAME = 'table_users' LIMIT 1);
SET @sql := IF(@fk IS NULL, 'DO 0', CONCAT('ALTER TABLE user_notification_subscriptions DROP FOREIGN KEY `', @fk, '`'));
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @fk := (SELECT CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_notification_channels'
    AND COLUMN_NAME = 'user' AND REFERENCED_TABLE_NAME = 'table_users' LIMIT 1);
SET @sql := IF(@fk IS NULL, 'DO 0', CONCAT('ALTER TABLE user_notification_channels DROP FOREIGN KEY `', @fk, '`'));
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- user_accounts.id is INT UNSIGNED; a foreign key needs the same type.
ALTER TABLE user_notification_channels MODIFY user INT UNSIGNED NOT NULL;
ALTER TABLE user_notification_subscriptions MODIFY user INT UNSIGNED NOT NULL;
-- Added only when missing: the down migration keeps them, so up can run again.
SET @sql := IF(EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_notification_channels_user_account'),
  'DO 0',
  'ALTER TABLE user_notification_channels ADD CONSTRAINT fk_notification_channels_user_account FOREIGN KEY (user) REFERENCES user_accounts(id) ON DELETE CASCADE');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_notification_subscriptions_user_account'),
  'DO 0',
  'ALTER TABLE user_notification_subscriptions ADD CONSTRAINT fk_notification_subscriptions_user_account FOREIGN KEY (user) REFERENCES user_accounts(id) ON DELETE CASCADE');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
