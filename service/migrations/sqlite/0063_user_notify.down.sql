-- The user_accounts foreign keys stay: pointing them back at table_users would break channels
-- for users created since armature-iam.
DELETE FROM rbac_role_permissions WHERE permission_id IN (
  SELECT id FROM rbac_permissions WHERE name = 'notifications.send'
);
DELETE FROM rbac_permissions WHERE name = 'notifications.send';
DELETE FROM notification_events WHERE event_code = 'user.notified';
